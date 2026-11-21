package rds

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	"github.com/convox/convox/provider/aws/provisioner"
	"github.com/convox/logger"
	"github.com/stretchr/testify/require"
)

const (
	testStateId = "pg-rrack1r-app1"

	queryErrorXML = `<ErrorResponse><Error><Type>Sender</Type><Code>%s</Code><Message>%s</Message></Error><RequestId>r1</RequestId></ErrorResponse>`
	ec2ErrorXML   = `<Response><Errors><Error><Code>%s</Code><Message>%s</Message></Error></Errors><RequestID>r1</RequestID></Response>`

	dbInstanceXML = `<DescribeDBInstancesResponse><DescribeDBInstancesResult><DBInstances><DBInstance>` +
		`<DBInstanceIdentifier>db1</DBInstanceIdentifier><DBInstanceStatus>%s</DBInstanceStatus><DeletionProtection>%t</DeletionProtection>` +
		`<TagList><Tag><Key>convox-rds</Key><Value>%s</Value></Tag></TagList>` +
		`</DBInstance></DBInstances></DescribeDBInstancesResult></DescribeDBInstancesResponse>`
	deleteDBInstanceXML  = `<DeleteDBInstanceResponse><DeleteDBInstanceResult><DBInstance><DBInstanceIdentifier>db1</DBInstanceIdentifier><DBInstanceStatus>deleting</DBInstanceStatus></DBInstance></DeleteDBInstanceResult></DeleteDBInstanceResponse>`
	deleteSubnetGroupXML = `<DeleteDBSubnetGroupResponse><ResponseMetadata><RequestId>r1</RequestId></ResponseMetadata></DeleteDBSubnetGroupResponse>`
	securityGroupsXML    = `<DescribeSecurityGroupsResponse><securityGroupInfo><item><groupId>sg-1</groupId></item></securityGroupInfo></DescribeSecurityGroupsResponse>`
	noSecurityGroupsXML  = `<DescribeSecurityGroupsResponse><securityGroupInfo/></DescribeSecurityGroupsResponse>`
	deleteSecurityGrpXML = `<DeleteSecurityGroupResponse><return>true</return></DeleteSecurityGroupResponse>`
)

type awsCall struct {
	action string
	form   url.Values
}

type awsReply struct {
	code int
	body string
}

type fakeAWS struct {
	mu      sync.Mutex
	calls   []awsCall
	respond map[string]awsReply
}

func (f *fakeAWS) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	action := r.Form.Get("Action")

	f.mu.Lock()
	f.calls = append(f.calls, awsCall{action: action, form: r.Form})
	f.mu.Unlock()

	out, ok := f.respond[action]
	if !ok {
		out = reply(http.StatusBadRequest, fmt.Sprintf(queryErrorXML, "UnknownOperation", "unexpected action"))
	}

	w.Header().Set("Content-Type", "text/xml")
	w.WriteHeader(out.code)
	_, _ = io.WriteString(w, out.body)
}

func (f *fakeAWS) actions() []string {
	f.mu.Lock()
	defer f.mu.Unlock()

	out := []string{}
	for _, c := range f.calls {
		out = append(out, c.action)
	}
	return out
}

func (f *fakeAWS) form(action string) url.Values {
	f.mu.Lock()
	defer f.mu.Unlock()

	for _, c := range f.calls {
		if c.action == action {
			return c.form
		}
	}
	return nil
}

type fakeStorage map[string][]byte

func (s fakeStorage) GetState(id string) ([]byte, error) {
	if data, ok := s[id]; ok {
		return data, nil
	}
	return nil, fmt.Errorf("state not found")
}

func (s fakeStorage) SaveState(string, []byte, string, map[string]string) error { return nil }

func (s fakeStorage) SendStateLog(string, string) error { return nil }

func reply(code int, body string) awsReply {
	return awsReply{code: code, body: body}
}

func instance(status string, protected bool, owner string) awsReply {
	return reply(http.StatusOK, fmt.Sprintf(dbInstanceXML, status, protected, owner))
}

func newUninstallProvisioner(t *testing.T, f *fakeAWS, subnetGroup string) *Provisioner {
	srv := httptest.NewServer(f)
	t.Cleanup(srv.Close)

	cfg := aws.Config{
		Region:           "us-east-1",
		Credentials:      aws.AnonymousCredentials{},
		BaseEndpoint:     aws.String(srv.URL),
		RetryMaxAttempts: 1,
	}

	state := NewState(testStateId, StateProvisioning, []Parameter{
		*NewParameter(ParamDBInstanceIdentifier, "db1", &MetaData{}),
		*NewParameter(ParamDBSubnetGroupName, subnetGroup, &MetaData{}),
	})

	data, err := state.GetStateInBytes()
	require.NoError(t, err)

	return &Provisioner{
		rdsClient: rds.NewFromConfig(cfg),
		ec2Client: ec2.NewFromConfig(cfg),
		storage:   fakeStorage{testStateId: data},
		logger:    logger.New("ns=rds-uninstall-test"),
	}
}

func TestUninstall(t *testing.T) {
	gone := reply(http.StatusNotFound, fmt.Sprintf(queryErrorXML, "DBInstanceNotFound", "DBInstance db1 is gone"))
	managedSubnet := "db-subg-" + testStateId

	tests := []struct {
		name        string
		subnetGroup string
		respond     map[string]awsReply
		wantErr     error
		wantActions []string
	}{
		{
			name:        "available instance starts the delete and waits",
			subnetGroup: managedSubnet,
			respond: map[string]awsReply{
				"DescribeDBInstances": instance("available", false, testStateId),
				"DeleteDBInstance":    reply(http.StatusOK, deleteDBInstanceXML),
			},
			wantErr:     provisioner.ErrDeleteInProgress,
			wantActions: []string{"DescribeDBInstances", "DescribeDBInstances", "DeleteDBInstance"},
		},
		{
			name:        "deleting instance waits without a second delete",
			subnetGroup: managedSubnet,
			respond: map[string]awsReply{
				"DescribeDBInstances": instance("deleting", false, testStateId),
			},
			wantErr:     provisioner.ErrDeleteInProgress,
			wantActions: []string{"DescribeDBInstances"},
		},
		{
			name:        "delete precheck waits without a second delete",
			subnetGroup: managedSubnet,
			respond: map[string]awsReply{
				"DescribeDBInstances": instance("delete-precheck", false, testStateId),
			},
			wantErr:     provisioner.ErrDeleteInProgress,
			wantActions: []string{"DescribeDBInstances"},
		},
		{
			name:        "deletion protection deletes nothing",
			subnetGroup: managedSubnet,
			respond: map[string]awsReply{
				"DescribeDBInstances": instance("available", true, testStateId),
			},
			wantActions: []string{"DescribeDBInstances"},
		},
		{
			name:        "gone instance deletes the security group and subnet group",
			subnetGroup: managedSubnet,
			respond: map[string]awsReply{
				"DescribeDBInstances":    gone,
				"DescribeSecurityGroups": reply(http.StatusOK, securityGroupsXML),
				"DeleteSecurityGroup":    reply(http.StatusOK, deleteSecurityGrpXML),
				"DeleteDBSubnetGroup":    reply(http.StatusOK, deleteSubnetGroupXML),
			},
			wantActions: []string{"DescribeDBInstances", "DescribeSecurityGroups", "DeleteSecurityGroup", "DeleteDBSubnetGroup"},
		},
		{
			name:        "security group in use still deletes the subnet group",
			subnetGroup: managedSubnet,
			respond: map[string]awsReply{
				"DescribeDBInstances":    gone,
				"DescribeSecurityGroups": reply(http.StatusOK, securityGroupsXML),
				"DeleteSecurityGroup":    reply(http.StatusBadRequest, fmt.Sprintf(ec2ErrorXML, "DependencyViolation", "resource sg-1 has a dependent object")),
				"DeleteDBSubnetGroup":    reply(http.StatusOK, deleteSubnetGroupXML),
			},
			wantErr:     provisioner.ErrGroupDelete,
			wantActions: []string{"DescribeDBInstances", "DescribeSecurityGroups", "DeleteSecurityGroup", "DeleteDBSubnetGroup"},
		},
		{
			name:        "subnet group in use is a group error",
			subnetGroup: managedSubnet,
			respond: map[string]awsReply{
				"DescribeDBInstances":    gone,
				"DescribeSecurityGroups": reply(http.StatusOK, noSecurityGroupsXML),
				"DeleteDBSubnetGroup":    reply(http.StatusBadRequest, fmt.Sprintf(queryErrorXML, "InvalidDBSubnetGroupStateFault", "in use")),
			},
			wantErr:     provisioner.ErrGroupDelete,
			wantActions: []string{"DescribeDBInstances", "DescribeSecurityGroups", "DeleteDBSubnetGroup"},
		},
		{
			name:        "subnet group already deleted",
			subnetGroup: managedSubnet,
			respond: map[string]awsReply{
				"DescribeDBInstances":    gone,
				"DescribeSecurityGroups": reply(http.StatusOK, noSecurityGroupsXML),
				"DeleteDBSubnetGroup":    reply(http.StatusNotFound, fmt.Sprintf(queryErrorXML, "DBSubnetGroupNotFoundFault", "DB subnet group is gone")),
			},
			wantActions: []string{"DescribeDBInstances", "DescribeSecurityGroups", "DeleteDBSubnetGroup"},
		},
		{
			name:        "subnet group the provisioner did not create is kept",
			subnetGroup: "my-subnet-group",
			respond: map[string]awsReply{
				"DescribeDBInstances":    gone,
				"DescribeSecurityGroups": reply(http.StatusOK, noSecurityGroupsXML),
			},
			wantActions: []string{"DescribeDBInstances", "DescribeSecurityGroups"},
		},
		{
			name:        "instance the provisioner did not create is kept",
			subnetGroup: "my-subnet-group",
			respond: map[string]awsReply{
				"DescribeDBInstances":    instance("available", false, "someone-else"),
				"DescribeSecurityGroups": reply(http.StatusOK, noSecurityGroupsXML),
			},
			wantActions: []string{"DescribeDBInstances", "DescribeSecurityGroups"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := &fakeAWS{respond: tc.respond}
			p := newUninstallProvisioner(t, f, tc.subnetGroup)

			err := p.Uninstall(testStateId)
			if tc.wantErr == nil {
				require.NoError(t, err)
			} else {
				require.ErrorIs(t, err, tc.wantErr)
			}
			require.Equal(t, tc.wantActions, f.actions())

			if form := f.form("DescribeSecurityGroups"); form != nil {
				require.Equal(t, "tag:convox-rds", form.Get("Filter.1.Name"))
				require.Equal(t, testStateId, form.Get("Filter.1.Value.1"))
			}
			if form := f.form("DeleteSecurityGroup"); form != nil {
				require.Equal(t, "sg-1", form.Get("GroupId"))
			}
			if form := f.form("DeleteDBSubnetGroup"); form != nil {
				require.Equal(t, managedSubnet, form.Get("DBSubnetGroupName"))
			}
			if form := f.form("DeleteDBInstance"); form != nil {
				require.True(t, strings.HasPrefix(form.Get("FinalDBSnapshotIdentifier"), "db1-final-snapshot-"))
				require.Empty(t, form.Get("SkipFinalSnapshot"))
			}
		})
	}
}

func TestGetDBInstanceNotFound(t *testing.T) {
	f := &fakeAWS{respond: map[string]awsReply{
		"DescribeDBInstances": reply(http.StatusNotFound, fmt.Sprintf(queryErrorXML, "DBInstanceNotFound", "DBInstance db1 is gone")),
	}}
	p := newUninstallProvisioner(t, f, "")

	_, err := p.GetDBInstance("db1")
	require.EqualError(t, err, "db instance not found")
}
