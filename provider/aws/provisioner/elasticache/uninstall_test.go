package elasticache

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
	"github.com/aws/aws-sdk-go-v2/service/elasticache"
	"github.com/convox/convox/provider/aws/provisioner"
	"github.com/convox/logger"
	"github.com/stretchr/testify/require"
)

const (
	testStateId = "cache-rrack1r-app1"

	queryErrorXML = `<ErrorResponse><Error><Type>Sender</Type><Code>%s</Code><Message>%s</Message></Error><RequestId>r1</RequestId></ErrorResponse>`
	ec2ErrorXML   = `<Response><Errors><Error><Code>%s</Code><Message>%s</Message></Error></Errors><RequestID>r1</RequestID></Response>`

	replicationGroupXML = `<DescribeReplicationGroupsResponse><DescribeReplicationGroupsResult><ReplicationGroups><ReplicationGroup>` +
		`<ReplicationGroupId>rg1</ReplicationGroupId><Status>%s</Status><ARN>arn:aws:elasticache:us-east-1:123456789012:replicationgroup:rg1</ARN>` +
		`</ReplicationGroup></ReplicationGroups></DescribeReplicationGroupsResult></DescribeReplicationGroupsResponse>`
	cacheClusterXML = `<DescribeCacheClustersResponse><DescribeCacheClustersResult><CacheClusters><CacheCluster>` +
		`<CacheClusterId>mc1</CacheClusterId><CacheClusterStatus>%s</CacheClusterStatus><ARN>arn:aws:elasticache:us-east-1:123456789012:cluster:mc1</ARN>` +
		`</CacheCluster></CacheClusters></DescribeCacheClustersResult></DescribeCacheClustersResponse>`
	tagsXML                   = `<ListTagsForResourceResponse><ListTagsForResourceResult><TagList><Tag><Key>convox-elasticache</Key><Value>%s</Value></Tag></TagList></ListTagsForResourceResult></ListTagsForResourceResponse>`
	deleteReplicationGrpXML   = `<DeleteReplicationGroupResponse><DeleteReplicationGroupResult><ReplicationGroup><ReplicationGroupId>rg1</ReplicationGroupId><Status>deleting</Status></ReplicationGroup></DeleteReplicationGroupResult></DeleteReplicationGroupResponse>`
	deleteCacheClusterXML     = `<DeleteCacheClusterResponse><DeleteCacheClusterResult><CacheCluster><CacheClusterId>mc1</CacheClusterId><CacheClusterStatus>deleting</CacheClusterStatus></CacheCluster></DeleteCacheClusterResult></DeleteCacheClusterResponse>`
	deleteCacheSubnetGroupXML = `<DeleteCacheSubnetGroupResponse><ResponseMetadata><RequestId>r1</RequestId></ResponseMetadata></DeleteCacheSubnetGroupResponse>`
	securityGroupsXML         = `<DescribeSecurityGroupsResponse><securityGroupInfo><item><groupId>sg-1</groupId></item></securityGroupInfo></DescribeSecurityGroupsResponse>`
	noSecurityGroupsXML       = `<DescribeSecurityGroupsResponse><securityGroupInfo/></DescribeSecurityGroupsResponse>`
	deleteSecurityGrpXML      = `<DeleteSecurityGroupResponse><return>true</return></DeleteSecurityGroupResponse>`
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

type cacheCase struct {
	name        string
	engine      string
	subnetGroup string
	imported    bool
	protected   bool
	respond     map[string]awsReply
	wantErr     error
	wantActions []string
}

func newUninstallProvisioner(t *testing.T, f *fakeAWS, engine, subnetGroup string, imported, protected bool) *Provisioner {
	srv := httptest.NewServer(f)
	t.Cleanup(srv.Close)

	cfg := aws.Config{
		Region:           "us-east-1",
		Credentials:      aws.AnonymousCredentials{},
		BaseEndpoint:     aws.String(srv.URL),
		RetryMaxAttempts: 1,
	}

	state := NewStateForImport(testStateId)
	if !imported {
		state = NewState(testStateId, StateProvisioning, []Parameter{
			*NewParameter(ParamCacheSubnetGroupName, subnetGroup, &MetaData{}),
		})
	}
	state.AddOrUpdateParameter(*NewParameter(ParamEngine, engine, &MetaData{}))
	state.AddOrUpdateParameter(*NewParameter(ParamReplicationGroupId, "rg1", &MetaData{}))
	state.AddOrUpdateParameter(*NewParameter(ParamCacheClusterId, "mc1", &MetaData{}))
	if protected {
		state.AddOrUpdateParameter(*NewParameter(ParamDeletionProtection, "true", &MetaData{}))
	}

	data, err := state.GetStateInBytes()
	require.NoError(t, err)

	return &Provisioner{
		elasticacheClient: elasticache.NewFromConfig(cfg),
		ec2Client:         ec2.NewFromConfig(cfg),
		storage:           fakeStorage{testStateId: data},
		logger:            logger.New("ns=elasticache-uninstall-test"),
	}
}

func TestUninstall(t *testing.T) {
	rgGone := reply(http.StatusNotFound, fmt.Sprintf(queryErrorXML, "ReplicationGroupNotFoundFault", "ReplicationGroup rg1 is gone"))
	clusterGone := reply(http.StatusNotFound, fmt.Sprintf(queryErrorXML, "CacheClusterNotFound", "CacheCluster mc1 is gone"))
	notAvailable := reply(http.StatusBadRequest, fmt.Sprintf(queryErrorXML, "InvalidReplicationGroupState", "not available"))
	managedSubnet := "cache-subg-" + testStateId

	group := func(status string) awsReply {
		return reply(http.StatusOK, fmt.Sprintf(replicationGroupXML, status))
	}
	cluster := func(status string) awsReply {
		return reply(http.StatusOK, fmt.Sprintf(cacheClusterXML, status))
	}
	tags := func(owner string) awsReply {
		return reply(http.StatusOK, fmt.Sprintf(tagsXML, owner))
	}

	tests := []cacheCase{
		{
			name:   "available replication group starts the delete and waits",
			engine: "redis",
			respond: map[string]awsReply{
				"DescribeReplicationGroups": group("available"),
				"ListTagsForResource":       tags(testStateId),
				"DeleteReplicationGroup":    reply(http.StatusOK, deleteReplicationGrpXML),
			},
			wantErr:     provisioner.ErrDeleteInProgress,
			wantActions: []string{"DescribeReplicationGroups", "ListTagsForResource", "DeleteReplicationGroup"},
		},
		{
			name:   "deleting replication group waits without reading tags",
			engine: "redis",
			respond: map[string]awsReply{
				"DescribeReplicationGroups": group("deleting"),
				"ListTagsForResource":       notAvailable,
			},
			wantErr:     provisioner.ErrDeleteInProgress,
			wantActions: []string{"DescribeReplicationGroups"},
		},
		{
			name:   "gone replication group deletes the security group and subnet group",
			engine: "redis",
			respond: map[string]awsReply{
				"DescribeReplicationGroups": rgGone,
				"DescribeSecurityGroups":    reply(http.StatusOK, securityGroupsXML),
				"DeleteSecurityGroup":       reply(http.StatusOK, deleteSecurityGrpXML),
				"DeleteCacheSubnetGroup":    reply(http.StatusOK, deleteCacheSubnetGroupXML),
			},
			wantActions: []string{"DescribeReplicationGroups", "DescribeSecurityGroups", "DeleteSecurityGroup", "DeleteCacheSubnetGroup"},
		},
		{
			name:   "security group in use still deletes the subnet group",
			engine: "redis",
			respond: map[string]awsReply{
				"DescribeReplicationGroups": rgGone,
				"DescribeSecurityGroups":    reply(http.StatusOK, securityGroupsXML),
				"DeleteSecurityGroup":       reply(http.StatusBadRequest, fmt.Sprintf(ec2ErrorXML, "DependencyViolation", "resource sg-1 has a dependent object")),
				"DeleteCacheSubnetGroup":    reply(http.StatusOK, deleteCacheSubnetGroupXML),
			},
			wantErr:     provisioner.ErrGroupDelete,
			wantActions: []string{"DescribeReplicationGroups", "DescribeSecurityGroups", "DeleteSecurityGroup", "DeleteCacheSubnetGroup"},
		},
		{
			name:   "subnet group in use is a group error",
			engine: "redis",
			respond: map[string]awsReply{
				"DescribeReplicationGroups": rgGone,
				"DescribeSecurityGroups":    reply(http.StatusOK, noSecurityGroupsXML),
				"DeleteCacheSubnetGroup":    reply(http.StatusBadRequest, fmt.Sprintf(queryErrorXML, "CacheSubnetGroupInUse", "in use")),
			},
			wantErr:     provisioner.ErrGroupDelete,
			wantActions: []string{"DescribeReplicationGroups", "DescribeSecurityGroups", "DeleteCacheSubnetGroup"},
		},
		{
			name:   "subnet group already deleted",
			engine: "redis",
			respond: map[string]awsReply{
				"DescribeReplicationGroups": rgGone,
				"DescribeSecurityGroups":    reply(http.StatusOK, noSecurityGroupsXML),
				"DeleteCacheSubnetGroup":    reply(http.StatusNotFound, fmt.Sprintf(queryErrorXML, "CacheSubnetGroupNotFoundFault", "CacheSubnetGroup is gone")),
			},
			wantActions: []string{"DescribeReplicationGroups", "DescribeSecurityGroups", "DeleteCacheSubnetGroup"},
		},
		{
			name:     "imported replication group is kept",
			engine:   "redis",
			imported: true,
			respond: map[string]awsReply{
				"DescribeReplicationGroups": group("available"),
				"ListTagsForResource":       tags("someone-else"),
				"DescribeSecurityGroups":    reply(http.StatusOK, noSecurityGroupsXML),
			},
			wantActions: []string{"DescribeReplicationGroups", "ListTagsForResource", "DescribeSecurityGroups"},
		},
		{
			name:        "deletion protection deletes nothing",
			engine:      "redis",
			protected:   true,
			wantActions: []string{},
		},
		{
			name:   "available cache cluster starts the delete and waits",
			engine: "memcached",
			respond: map[string]awsReply{
				"DescribeCacheClusters": cluster("available"),
				"ListTagsForResource":   tags(testStateId),
				"DeleteCacheCluster":    reply(http.StatusOK, deleteCacheClusterXML),
			},
			wantErr:     provisioner.ErrDeleteInProgress,
			wantActions: []string{"DescribeCacheClusters", "ListTagsForResource", "DeleteCacheCluster"},
		},
		{
			name:   "deleting cache cluster waits without reading tags",
			engine: "memcached",
			respond: map[string]awsReply{
				"DescribeCacheClusters": cluster("deleting"),
				"ListTagsForResource":   notAvailable,
			},
			wantErr:     provisioner.ErrDeleteInProgress,
			wantActions: []string{"DescribeCacheClusters"},
		},
		{
			name:   "deleted cache cluster waits without reading tags",
			engine: "memcached",
			respond: map[string]awsReply{
				"DescribeCacheClusters": cluster("deleted"),
				"ListTagsForResource":   notAvailable,
			},
			wantErr:     provisioner.ErrDeleteInProgress,
			wantActions: []string{"DescribeCacheClusters"},
		},
		{
			name:   "gone cache cluster deletes the security group and subnet group",
			engine: "memcached",
			respond: map[string]awsReply{
				"DescribeCacheClusters":  clusterGone,
				"DescribeSecurityGroups": reply(http.StatusOK, securityGroupsXML),
				"DeleteSecurityGroup":    reply(http.StatusOK, deleteSecurityGrpXML),
				"DeleteCacheSubnetGroup": reply(http.StatusOK, deleteCacheSubnetGroupXML),
			},
			wantActions: []string{"DescribeCacheClusters", "DescribeSecurityGroups", "DeleteSecurityGroup", "DeleteCacheSubnetGroup"},
		},
		{
			name:   "security group in use still deletes the cache cluster subnet group",
			engine: "memcached",
			respond: map[string]awsReply{
				"DescribeCacheClusters":  clusterGone,
				"DescribeSecurityGroups": reply(http.StatusOK, securityGroupsXML),
				"DeleteSecurityGroup":    reply(http.StatusBadRequest, fmt.Sprintf(ec2ErrorXML, "DependencyViolation", "resource sg-1 has a dependent object")),
				"DeleteCacheSubnetGroup": reply(http.StatusOK, deleteCacheSubnetGroupXML),
			},
			wantErr:     provisioner.ErrGroupDelete,
			wantActions: []string{"DescribeCacheClusters", "DescribeSecurityGroups", "DeleteSecurityGroup", "DeleteCacheSubnetGroup"},
		},
		{
			name:        "replication group subnet group the provisioner did not create is kept",
			engine:      "redis",
			subnetGroup: "my-subnet-group",
			respond: map[string]awsReply{
				"DescribeReplicationGroups": rgGone,
				"DescribeSecurityGroups":    reply(http.StatusOK, noSecurityGroupsXML),
			},
			wantActions: []string{"DescribeReplicationGroups", "DescribeSecurityGroups"},
		},
		{
			name:        "cache cluster subnet group the provisioner did not create is kept",
			engine:      "memcached",
			subnetGroup: "my-subnet-group",
			respond: map[string]awsReply{
				"DescribeCacheClusters":  clusterGone,
				"DescribeSecurityGroups": reply(http.StatusOK, noSecurityGroupsXML),
			},
			wantActions: []string{"DescribeCacheClusters", "DescribeSecurityGroups"},
		},
		{
			name:     "imported cache cluster is kept",
			engine:   "memcached",
			imported: true,
			respond: map[string]awsReply{
				"DescribeCacheClusters":  cluster("available"),
				"ListTagsForResource":    tags("someone-else"),
				"DescribeSecurityGroups": reply(http.StatusOK, noSecurityGroupsXML),
			},
			wantActions: []string{"DescribeCacheClusters", "ListTagsForResource", "DescribeSecurityGroups"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := &fakeAWS{respond: tc.respond}
			subnetGroup := tc.subnetGroup
			if subnetGroup == "" {
				subnetGroup = managedSubnet
			}
			p := newUninstallProvisioner(t, f, tc.engine, subnetGroup, tc.imported, tc.protected)

			err := p.Uninstall(testStateId)
			if tc.wantErr == nil {
				require.NoError(t, err)
			} else {
				require.ErrorIs(t, err, tc.wantErr)
			}
			require.Equal(t, tc.wantActions, f.actions())

			if form := f.form("DescribeSecurityGroups"); form != nil {
				require.Equal(t, "tag:convox-elasticache", form.Get("Filter.1.Name"))
				require.Equal(t, testStateId, form.Get("Filter.1.Value.1"))
			}
			if form := f.form("DeleteSecurityGroup"); form != nil {
				require.Equal(t, "sg-1", form.Get("GroupId"))
			}
			if form := f.form("DeleteCacheSubnetGroup"); form != nil {
				require.Equal(t, managedSubnet, form.Get("CacheSubnetGroupName"))
			}
			if form := f.form("DeleteReplicationGroup"); form != nil {
				require.True(t, strings.HasPrefix(form.Get("FinalSnapshotIdentifier"), "rg1-final-snapshot-"))
			}
		})
	}
}

func TestGetNotFound(t *testing.T) {
	f := &fakeAWS{respond: map[string]awsReply{
		"DescribeReplicationGroups": reply(http.StatusNotFound, fmt.Sprintf(queryErrorXML, "ReplicationGroupNotFoundFault", "ReplicationGroup rg1 is gone")),
		"DescribeCacheClusters":     reply(http.StatusNotFound, fmt.Sprintf(queryErrorXML, "CacheClusterNotFound", "CacheCluster mc1 is gone")),
	}}
	p := newUninstallProvisioner(t, f, "redis", "", false, false)

	_, err := p.GetReplicationGroup("rg1")
	require.EqualError(t, err, "replication group not found")

	_, err = p.GetCacheCluster("mc1")
	require.EqualError(t, err, "cache cluster not found")
}
