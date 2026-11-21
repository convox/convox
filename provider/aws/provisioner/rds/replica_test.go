package rds

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	"github.com/convox/logger"
	"github.com/stretchr/testify/require"
)

const (
	replicaSourceId         = "mydb-rrack1r-app1"
	replicaSourceIdentifier = "mydb-rrack1r-app10123456789ab"
	replicaId               = "dbrr-rrack1r-app1"

	replicaErrorXML    = `<ErrorResponse><Error><Type>Sender</Type><Code>%s</Code><Message>%s</Message></Error><RequestId>r1</RequestId></ErrorResponse>`
	replicaCreateXML   = `<CreateDBInstanceReadReplicaResponse><CreateDBInstanceReadReplicaResult><DBInstance><DBInstanceIdentifier>rr1</DBInstanceIdentifier></DBInstance></CreateDBInstanceReadReplicaResult></CreateDBInstanceReadReplicaResponse>`
	replicaPromoteXML  = `<PromoteReadReplicaResponse><PromoteReadReplicaResult><DBInstance><DBInstanceIdentifier>rr1</DBInstanceIdentifier></DBInstance></PromoteReadReplicaResult></PromoteReadReplicaResponse>`
	replicaDescribeXML = `<DescribeDBInstancesResponse><DescribeDBInstancesResult><DBInstances><DBInstance>` +
		`<DBInstanceIdentifier>rr1</DBInstanceIdentifier><DBInstanceStatus>%s</DBInstanceStatus>%s` +
		`</DBInstance></DBInstances></DescribeDBInstancesResult></DescribeDBInstancesResponse>`
)

type replicaCall struct {
	action string
	form   url.Values
}

type replicaAWS struct {
	mu      sync.Mutex
	calls   []replicaCall
	respond map[string]string
}

func (f *replicaAWS) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	action := r.Form.Get("Action")

	f.mu.Lock()
	f.calls = append(f.calls, replicaCall{action: action, form: r.Form})
	body, ok := f.respond[action]
	f.mu.Unlock()

	code := http.StatusOK
	if !ok {
		code = http.StatusBadRequest
		body = fmt.Sprintf(replicaErrorXML, "UnknownOperation", "unexpected action")
	}

	w.Header().Set("Content-Type", "text/xml")
	w.WriteHeader(code)
	_, _ = io.WriteString(w, body)
}

func (f *replicaAWS) actions() []string {
	f.mu.Lock()
	defer f.mu.Unlock()

	out := []string{}
	for _, c := range f.calls {
		out = append(out, c.action)
	}
	return out
}

func (f *replicaAWS) form(action string) url.Values {
	f.mu.Lock()
	defer f.mu.Unlock()

	for _, c := range f.calls {
		if c.action == action {
			return c.form
		}
	}
	return nil
}

type replicaStorage struct {
	states map[string][]byte
	errs   map[string]error
	logs   []string
}

func (s *replicaStorage) GetState(id string) ([]byte, error) {
	if err := s.errs[id]; err != nil {
		return nil, err
	}
	if data, ok := s.states[id]; ok {
		return data, nil
	}
	return nil, fmt.Errorf("state not found")
}

func (s *replicaStorage) SaveState(id string, data []byte, _ string, _ map[string]string) error {
	s.states[id] = data
	return nil
}

func (s *replicaStorage) SendStateLog(_, message string) error {
	s.logs = append(s.logs, message)
	return nil
}

func (s *replicaStorage) state(t *testing.T, id string) *StateData {
	t.Helper()

	data, ok := s.states[id]
	require.True(t, ok, "no state saved for %s", id)

	state := NewEmptyState()
	require.NoError(t, state.LoadState(data))
	return state
}

func newReplicaProvisioner(t *testing.T, f *replicaAWS, s *replicaStorage) *Provisioner {
	t.Helper()

	srv := httptest.NewServer(f)
	t.Cleanup(srv.Close)

	cfg := aws.Config{
		Region:           "us-east-1",
		Credentials:      aws.AnonymousCredentials{},
		BaseEndpoint:     aws.String(srv.URL),
		RetryMaxAttempts: 1,
	}

	return &Provisioner{
		rdsClient: rds.NewFromConfig(cfg),
		ec2Client: ec2.NewFromConfig(cfg),
		storage:   s,
		logger:    logger.New("ns=rds-replica-test"),
	}
}

func replicaStateBytes(t *testing.T, s *StateData) []byte {
	t.Helper()

	data, err := s.GetStateInBytes()
	require.NoError(t, err)
	return data
}

func replicaSourceState(t *testing.T) []byte {
	return replicaStateBytes(t, NewState(replicaSourceId, StateProvisionSucceeded, []Parameter{
		*NewParameter(ParamDBInstanceIdentifier, replicaSourceIdentifier, &MetaData{}),
		*NewParameter(ParamVPCSecurityGroups, "sg-source", &MetaData{}),
		*NewParameter(ParamMasterUserPassword, "source-pass", &MetaData{}),
	}))
}

func replicaInstalledState(t *testing.T) *StateData {
	t.Helper()

	values := map[string]string{
		ParamDBInstanceIdentifier:       "rr1",
		ParamSourceDBInstanceIdentifier: replicaSourceId,
		ParamEngine:                     "postgres",
		ParamVPC:                        "vpc-1",
		ParamSubnetIds:                  "subnet-1,subnet-2",
		ParamDeletionProtection:         "false",
		ParamMasterUserPassword:         "source-pass",
	}

	params := []Parameter{}
	for k, m := range GetParametersMetaDataForReadReplicaInstall() {
		params = append(params, *NewParameter(k, values[k], m))
	}

	return NewState(replicaId, StateProvisionSucceeded, params)
}

func replicaPromotedState(t *testing.T) *StateData {
	t.Helper()

	s := replicaInstalledState(t)
	s.AddOrUpdateParameter(*NewParameterForValuePtr(ParamSourceDBInstanceIdentifier, nil, &MetaData{}))
	return s
}

func replicaDescribe(sourceIdentifier string) string {
	source := ""
	if sourceIdentifier != "" {
		source = fmt.Sprintf("<ReadReplicaSourceDBInstanceIdentifier>%s</ReadReplicaSourceDBInstanceIdentifier>", sourceIdentifier)
	}
	return fmt.Sprintf(replicaDescribeXML, "available", source)
}

func replicaOptions(extra map[string]string) map[string]string {
	options := map[string]string{
		ParamEngine:                     "postgres",
		ParamVPC:                        "vpc-1",
		ParamSubnetIds:                  "subnet-1,subnet-2",
		ParamSourceDBInstanceIdentifier: replicaSourceId,
	}
	for k, v := range extra {
		options[k] = v
	}
	return options
}

func TestInstallReplica(t *testing.T) {
	tests := []struct {
		name         string
		options      map[string]string
		states       map[string][]byte
		errs         map[string]error
		wantErr      string
		wantForm     map[string]string
		wantAbsent   []string
		wantPassword string
	}{
		{
			name:    "convox source resolves to its identifier and security group",
			options: replicaOptions(nil),
			wantForm: map[string]string{
				"SourceDBInstanceIdentifier":               replicaSourceIdentifier,
				"VpcSecurityGroupIds.VpcSecurityGroupId.1": "sg-source",
			},
			wantAbsent:   []string{"DBInstanceClass", "AllocatedStorage"},
			wantPassword: "source-pass",
		},
		{
			name: "replica options win over the source",
			options: replicaOptions(map[string]string{
				ParamDBInstanceClass:    "db.t3.large",
				ParamAllocatedStorage:   "200",
				ParamVPCSecurityGroups:  "sg-replica",
				ParamMasterUserPassword: "replica-pass",
			}),
			wantForm: map[string]string{
				"SourceDBInstanceIdentifier":               replicaSourceIdentifier,
				"DBInstanceClass":                          "db.t3.large",
				"AllocatedStorage":                         "200",
				"VpcSecurityGroupIds.VpcSecurityGroupId.1": "sg-replica",
			},
			wantPassword: "replica-pass",
		},
		{
			name:    "convox source without a security group",
			options: replicaOptions(nil),
			states: map[string][]byte{replicaSourceId: replicaStateBytes(t, NewState(replicaSourceId, StateProvisionSucceeded, []Parameter{
				*NewParameter(ParamDBInstanceIdentifier, replicaSourceIdentifier, &MetaData{}),
				*NewParameter(ParamMasterUserPassword, "source-pass", &MetaData{}),
			}))},
			wantForm: map[string]string{
				"SourceDBInstanceIdentifier": replicaSourceIdentifier,
			},
			wantAbsent:   []string{"VpcSecurityGroupIds.VpcSecurityGroupId.1"},
			wantPassword: "source-pass",
		},
		{
			name: "source outside convox passes through",
			options: replicaOptions(map[string]string{
				ParamSourceDBInstanceIdentifier: "external-db",
				ParamMasterUserPassword:         "external-pass",
			}),
			wantForm: map[string]string{
				"SourceDBInstanceIdentifier": "external-db",
			},
			wantAbsent:   []string{"VpcSecurityGroupIds.VpcSecurityGroupId.1"},
			wantPassword: "external-pass",
		},
		{
			name:    "source lookup error is returned",
			options: replicaOptions(map[string]string{ParamMasterUserPassword: "replica-pass"}),
			errs:    map[string]error{replicaSourceId: fmt.Errorf("connection refused")},
			wantErr: "connection refused",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := &replicaAWS{respond: map[string]string{"CreateDBInstanceReadReplica": replicaCreateXML}}
			states := tc.states
			if states == nil {
				states = map[string][]byte{replicaSourceId: replicaSourceState(t)}
			}
			s := &replicaStorage{states: states, errs: tc.errs}

			err := newReplicaProvisioner(t, f, s).InstallReplica(replicaId, tc.options, ProvisionExtraOpts{})
			if tc.wantErr != "" {
				require.ErrorContains(t, err, tc.wantErr)
				require.Empty(t, f.actions())
				return
			}
			require.NoError(t, err)

			form := f.form("CreateDBInstanceReadReplica")
			require.NotNil(t, form)
			for k, v := range tc.wantForm {
				require.Equal(t, v, form.Get(k), k)
			}
			for _, k := range tc.wantAbsent {
				require.False(t, form.Has(k), k)
			}

			state := s.state(t, replicaId)
			source, err := state.GetParameterValue(ParamSourceDBInstanceIdentifier)
			require.NoError(t, err)
			require.Equal(t, tc.options[ParamSourceDBInstanceIdentifier], source)

			pass, err := state.GetParameterValue(ParamMasterUserPassword)
			require.NoError(t, err)
			require.Equal(t, tc.wantPassword, pass)

			groups, err := state.GetParameterValuePtr(ParamVPCSecurityGroups)
			require.NoError(t, err)
			if v := tc.options[ParamVPCSecurityGroups]; v != "" {
				require.Equal(t, v, aws.ToString(groups))
			} else {
				require.Nil(t, groups)
			}
		})
	}
}

func TestUpdatePromotesReadReplica(t *testing.T) {
	promoteOptions := map[string]string{ParamEngine: "postgres"}

	tests := []struct {
		name        string
		state       func(t *testing.T) *StateData
		options     map[string]string
		describe    string
		wantActions []string
	}{
		{
			name:        "replica is promoted and recorded",
			state:       replicaInstalledState,
			options:     promoteOptions,
			describe:    replicaDescribe(replicaSourceIdentifier),
			wantActions: []string{"DescribeDBInstances", "PromoteReadReplica"},
		},
		{
			name:        "standalone instance is recorded without a promote",
			state:       replicaInstalledState,
			options:     promoteOptions,
			describe:    replicaDescribe(""),
			wantActions: []string{"DescribeDBInstances"},
		},
		{
			name:        "source option on a promoted replica is ignored",
			state:       replicaPromotedState,
			options:     replicaOptions(nil),
			wantActions: []string{},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := &replicaAWS{respond: map[string]string{
				"DescribeDBInstances": tc.describe,
				"PromoteReadReplica":  replicaPromoteXML,
			}}
			s := &replicaStorage{states: map[string][]byte{replicaId: replicaStateBytes(t, tc.state(t))}}

			require.NoError(t, newReplicaProvisioner(t, f, s).Update(replicaId, tc.options, ProvisionExtraOpts{}))
			require.Equal(t, tc.wantActions, f.actions())
			for _, action := range tc.wantActions {
				require.Equal(t, "rr1", f.form(action).Get("DBInstanceIdentifier"), action)
			}

			source := s.state(t, replicaId).GetParameter(ParamSourceDBInstanceIdentifier)
			require.NotNil(t, source)
			require.True(t, source.IsValueEmpty())
		})
	}
}

func TestUpdateRecordsPromotionBeforeOptionChanges(t *testing.T) {
	f := &replicaAWS{respond: map[string]string{
		"DescribeDBInstances": replicaDescribe(replicaSourceIdentifier),
		"PromoteReadReplica":  replicaPromoteXML,
	}}
	s := &replicaStorage{states: map[string][]byte{replicaId: replicaStateBytes(t, replicaInstalledState(t))}}
	p := newReplicaProvisioner(t, f, s)
	options := map[string]string{ParamEngine: "postgres", ParamDeletionProtection: "true"}

	require.ErrorContains(t, p.Update(replicaId, options, ProvisionExtraOpts{}), "modification request failed")
	require.Equal(t, []string{"DescribeDBInstances", "PromoteReadReplica", "ModifyDBInstance"}, f.actions())

	state := s.state(t, replicaId)
	require.True(t, state.GetParameter(ParamSourceDBInstanceIdentifier).IsValueEmpty())
	protection, err := state.GetParameterValue(ParamDeletionProtection)
	require.NoError(t, err)
	require.Equal(t, "false", protection)

	require.ErrorContains(t, p.Update(replicaId, options, ProvisionExtraOpts{}), "modification request failed")
	require.Equal(t, []string{"DescribeDBInstances", "PromoteReadReplica", "ModifyDBInstance", "ModifyDBInstance"}, f.actions())
}

func TestWaitUntilTargetDBStatusChecksBeforeWaiting(t *testing.T) {
	f := &replicaAWS{respond: map[string]string{"DescribeDBInstances": replicaDescribe("")}}
	p := newReplicaProvisioner(t, f, &replicaStorage{})

	err := p.waitUntilTargetDBStatus("rr1", &DBStatusWaiterConf{
		Pending: DbInstanceCreatePendingStates,
		Target:  DbInstanceAvailableStates,
		Timeout: 5 * time.Second,
		Delay:   time.Hour,
	})
	require.NoError(t, err)
	require.Equal(t, []string{"DescribeDBInstances"}, f.actions())
}
