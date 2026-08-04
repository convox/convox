package k8s

import (
	"context"
	"testing"

	"github.com/convox/convox/pkg/manifest"
	"github.com/convox/convox/pkg/options"
	"github.com/convox/convox/pkg/structs"
	"github.com/convox/logger"
	"github.com/stretchr/testify/require"
	ac "k8s.io/api/core/v1"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
)

const rdsOptionTemplate = `{
	"postgres": [
		{"ParamName": "class", "AllowedValues": ["small", "medium"], "MapAllowedToOriginal": {"small": "db.t4g.small", "medium": "db.t4g.medium"}},
		{"ParamName": "storage", "AllowedValues": ["small", "medium"], "MapAllowedToOriginal": {"small": "20", "medium": "50"}},
		{"ParamName": "version", "AllowedValues": ["16", "17"], "Default": "16"},
		{"ParamName": "encrypted", "AllowedValues": ["false"], "Default": "false"},
		{"ParamName": "backupRetentionPeriod", "AllowedMinimum": 1, "AllowedMaximum": 7}
	]
}`

func newTestAwsProvider(objs ...runtime.Object) *Provider {
	return &Provider{
		Cluster:      fake.NewSimpleClientset(objs...),
		FeatureGates: map[string]bool{},
		Name:         "rack1",
		RackName:     "rack1",
		Namespace:    "rack1",
		ctx:          context.Background(),
		logger:       logger.New("ns=aws-resources-test"),
	}
}

func tidContext(tid string) context.Context {
	return context.WithValue(context.Background(), structs.ConvoxTIDCtxKey, tid)
}

func mkStateSecret(ns, name string, labels map[string]string, data map[string][]byte) *ac.Secret {
	return &ac.Secret{
		ObjectMeta: am.ObjectMeta{Namespace: ns, Name: name, Labels: labels},
		Data:       data,
	}
}

func TestGenerateResourceStateId(t *testing.T) {
	require.Equal(t, "pg-rrack1r-app1", generateResourceStateId("rack1", "", "app1", "pg"))
	require.Equal(t, "pg-ab12-app1", generateResourceStateId("rack1", "ab12", "app1", "pg"))
}

func TestCreateAwsResourceStateId(t *testing.T) {
	p := newTestAwsProvider()

	id, err := p.CreateAwsResourceStateId("", "app1", "pg")
	require.NoError(t, err)
	require.Equal(t, "pg-rrack1r-app1", id)

	all, err := p.Cluster.CoreV1().Secrets(ac.NamespaceAll).List(context.TODO(), am.ListOptions{})
	require.NoError(t, err)
	require.Empty(t, all.Items, "a rack keyed id must not create a secret in any namespace")

	id, err = p.CreateAwsResourceStateId("ab12", "app1", "pg")
	require.NoError(t, err)
	require.Equal(t, "pg-ab12-app1", id)

	s, err := p.Cluster.CoreV1().Secrets("rack1-ab12-app1").Get(context.TODO(), id, am.GetOptions{})
	require.NoError(t, err)
	require.Equal(t, map[string]string{
		"rack":     "rack1",
		"system":   "convox",
		"app":      "app1",
		"resource": "pg",
		"tid":      "ab12",
	}, s.Labels)

	_, err = p.CreateAwsResourceStateId("ab12", "app1", "pg")
	require.NoError(t, err, "creating the same id twice must be idempotent")

	now := am.Now()
	s.DeletionTimestamp = &now
	_, err = p.Cluster.CoreV1().Secrets("rack1-ab12-app1").Update(context.TODO(), s, am.UpdateOptions{})
	require.NoError(t, err)

	_, err = p.CreateAwsResourceStateId("ab12", "app1", "pg")
	require.EqualError(t, err, "resource pg is still being deleted, promote again once it is gone")
}

// The provisioner storage callbacks run on the base provider, so SaveState has no
// tenant context: it must still land in the tenant namespace.
func TestSaveStateWithoutTenantContext(t *testing.T) {
	p := newTestAwsProvider(mkStateSecret("rack1-ab12-app1", "pg-ab12-app1", map[string]string{
		"rack": "rack1", "system": "convox", "app": "app1", "resource": "pg", "tid": "ab12",
	}, nil))

	require.Equal(t, "", p.ContextTID())

	meta := map[string]string{MetaAppKey: "app1", MetaResourceKey: "pg", MetaTidKey: "ab12"}
	require.NoError(t, p.SaveState("pg-ab12-app1", []byte(`{"id":"pg"}`), "convox-rds", meta))

	s, err := p.Cluster.CoreV1().Secrets("rack1-ab12-app1").Get(context.TODO(), "pg-ab12-app1", am.GetOptions{})
	require.NoError(t, err)
	require.Equal(t, []byte(`{"id":"pg"}`), s.Data[StateDataKey])
	require.Equal(t, "state", s.Labels["type"])
	require.Equal(t, "ab12", s.Labels["tid"])
	require.Equal(t, "app1", s.Labels["app"])
	require.Equal(t, "pg", s.Labels["resource"])
	require.Contains(t, s.Finalizers, StateFinalizer)

	_, err = p.Cluster.CoreV1().Secrets("rack1-app1").Get(context.TODO(), "pg-ab12-app1", am.GetOptions{})
	require.Error(t, err, "no state may be written to the non tenant namespace")
}

func TestSaveStateRackKeyed(t *testing.T) {
	p := newTestAwsProvider()

	require.NoError(t, p.SaveState("pg-rrack1r-app1", []byte(`{"id":"pg"}`), "convox-rds", nil))

	s, err := p.Cluster.CoreV1().Secrets("rack1-app1").Get(context.TODO(), "pg-rrack1r-app1", am.GetOptions{})
	require.NoError(t, err)
	require.Equal(t, []byte(`{"id":"pg"}`), s.Data[StateDataKey])
	require.Equal(t, "app1", s.Labels["app"])
	require.Equal(t, "pg", s.Labels["resource"])
	require.Equal(t, "", s.Labels["tid"])
}

func TestGetStateAndInfo(t *testing.T) {
	p := newTestAwsProvider(
		mkStateSecret("rack1-ab12-app1", "pg-ab12-app1", map[string]string{
			"rack": "rack1", "system": "convox", "type": "state", "app": "app1", "resource": "pg", "tid": "ab12",
		}, map[string][]byte{StateDataKey: []byte("tenant")}),
		mkStateSecret("rack1-app2", "pg-rrack1r-app2", map[string]string{
			"rack": "rack1", "system": "convox", "type": "state",
		}, map[string][]byte{StateDataKey: []byte("rack")}),
	)

	data, err := p.GetState("pg-ab12-app1")
	require.NoError(t, err)
	require.Equal(t, []byte("tenant"), data)

	info, err := p.GetInfoFromAwsResourceStateId("pg-ab12-app1")
	require.NoError(t, err)
	require.Equal(t, &AwsStateIdInfo{App: "app1", ResourceName: "pg", Tid: "ab12"}, info)

	data, err = p.GetState("pg-rrack1r-app2")
	require.NoError(t, err)
	require.Equal(t, []byte("rack"), data)

	info, err = p.GetInfoFromAwsResourceStateId("pg-rrack1r-app2")
	require.NoError(t, err)
	require.Equal(t, &AwsStateIdInfo{App: "app2", ResourceName: "pg"}, info)

	_, err = p.GetState("no-such-state")
	require.EqualError(t, err, "state not found")

	_, err = p.GetInfoFromAwsResourceStateId("no-such-state")
	require.EqualError(t, err, "state not found")
}

func TestMapToRdsParameterUngated(t *testing.T) {
	p := newTestAwsProvider()
	p.VpcID = "vpc-rack"

	out := p.MapToRdsParameter("rds-postgres", "app1", map[string]string{
		"class":   "db.t4g.small",
		"storage": "20",
		"vpc":     "vpc-manifest",
	})

	require.Equal(t, "postgres", out["Engine"])
	require.Equal(t, "db.t4g.small", out["DBInstanceClass"])
	require.Equal(t, "20", out["AllocatedStorage"])
	require.Equal(t, "vpc-manifest", out["VPC"], "the uncurated path still honors a manifest vpc")
}

func TestMapToRdsParameterAndMeta(t *testing.T) {
	p := newTestAwsProvider(&ac.ConfigMap{
		ObjectMeta: am.ObjectMeta{Namespace: "rack1", Name: "rds-template"},
		Data:       map[string]string{"config": rdsOptionTemplate},
	})
	p.ctx = tidContext("ab12")
	p.VpcID = "vpc-rack"
	p.SubnetIDs = "subnet-rack"
	t.Setenv("FEATURE_GATES", "rds-template-config=rds-template")

	r := manifest.Resource{Name: "pg", Type: "rds-postgres", Options: map[string]string{
		"class":    "Small",
		"iops":     "3000",
		"vpc":      "vpc-tenant",
		"subnets":  "subnet-tenant",
		MetaTidKey: "forged",
	}}

	out, meta, err := p.MapToRdsParameterAndMeta("ab12", "rds-postgres", "app1", r)
	require.NoError(t, err)

	require.Equal(t, "db.t4g.small", out["DBInstanceClass"], "friendly class maps to the real instance class")
	require.Equal(t, "20", out["AllocatedStorage"], "storage is pinned to the class")
	require.Equal(t, "16", out["EngineVersion"], "template default is applied")
	require.Equal(t, "false", out["StorageEncrypted"])
	require.Equal(t, "vpc-rack", out["VPC"], "a curated resource goes in the rack's vpc, not the manifest's")
	require.Equal(t, "subnet-rack", out["SubnetIds"])
	require.NotContains(t, out, "Iops", "an option the template does not declare is dropped")

	require.Equal(t, "small", meta["class"], "meta records the requested tier, not the mapped instance class")
	require.Equal(t, "small", meta["storage"])
	require.NotContains(t, meta, "iops", "an option the template does not declare is not recorded either")
	require.Equal(t, "app1", meta[MetaAppKey])
	require.Equal(t, "pg", meta[MetaResourceKey])
	require.Equal(t, "rack1", meta[MetaRackKey])
	require.Equal(t, "ab12", meta[MetaTidKey], "a manifest option cannot forge the tenant")

	r.Options["class"] = "xlarge"
	_, _, err = p.MapToRdsParameterAndMeta("ab12", "rds-postgres", "app1", r)
	require.EqualError(t, err, "db options class: value 'xlarge' is not allowed")

	r.Options["class"] = "small"
	r.Options["backupRetentionPeriod"] = "30"
	_, _, err = p.MapToRdsParameterAndMeta("ab12", "rds-postgres", "app1", r)
	require.EqualError(t, err, "db options backupRetentionPeriod: value '30' is above the allowed maximum 7")

	r.Options["backupRetentionPeriod"] = "0"
	_, _, err = p.MapToRdsParameterAndMeta("ab12", "rds-postgres", "app1", r)
	require.EqualError(t, err, "db options backupRetentionPeriod: value '0' is below the allowed minimum 1")

	r.Options["backupRetentionPeriod"] = "not-a-number"
	_, _, err = p.MapToRdsParameterAndMeta("ab12", "rds-postgres", "app1", r)
	require.EqualError(t, err, "db options backupRetentionPeriod: value 'not-a-number' is not a valid integer")

	delete(r.Options, "backupRetentionPeriod")
	_, _, err = p.MapToRdsParameterAndMeta("ab12", "rds-mysql", "app1", r)
	require.EqualError(t, err, "db type 'mysql' is not supported")
}

func TestAwsResourceMetaUngated(t *testing.T) {
	p := newTestAwsProvider()

	r := manifest.Resource{Name: "pg", Type: "rds-postgres", Options: map[string]string{
		"class":              "db.t4g.small",
		"durable":            "true",
		"masterUserPassword": "hunter2",
		"password":           "hunter2",
		MetaTidKey:           "forged",
		MetaAppKey:           "forged",
	}}

	meta := p.awsResourceMeta("ab12", "app1", r, nil)

	require.Equal(t, "db.t4g.small", meta["class"])
	require.Equal(t, "true", meta["durable"])
	require.Equal(t, "app1", meta[MetaAppKey], "a manifest option cannot forge the app")
	require.Equal(t, "ab12", meta[MetaTidKey], "a manifest option cannot forge the tenant")
	require.NotContains(t, meta, "masterUserPassword", "credential options are not recorded")
	require.NotContains(t, meta, "password", "credential options are not recorded")
}

func TestValidateAwsProviderResource(t *testing.T) {
	p := newTestAwsProvider()

	require.NoError(t, p.validateAwsProviderResource(manifest.Resource{Name: "pg", Type: "rds-postgres"}))
	require.NoError(t, p.validateAwsProviderResource(manifest.Resource{Name: "pg", Type: "postgres"}))

	err := p.validateAwsProviderResource(manifest.Resource{Name: "pg", Type: "postgres", Provider: "aws"})
	require.EqualError(t, err, "resource pg: provider aws is not supported on this rack")

	p.FeatureGates[options.FeatureGateRDSTemplateConfig] = true

	require.NoError(t, p.validateAwsProviderResource(manifest.Resource{Name: "pg", Type: "postgres", Provider: "aws"}))
	require.NoError(t, p.validateAwsProviderResource(manifest.Resource{Name: "cache", Type: "redis", Provider: "aws"}))

	err = p.validateAwsProviderResource(manifest.Resource{Name: "gis", Type: "postgis", Provider: "aws"})
	require.EqualError(t, err, "resource gis: provider aws is not supported for type postgis")
}

func TestAwsResourceType(t *testing.T) {
	require.Equal(t, "rds-postgres", awsResourceType("rds-", "postgres"))
	require.Equal(t, "rds-postgres", awsResourceType("rds-", "rds-postgres"))
	require.Equal(t, "elasticache-redis", awsResourceType("elasticache-", "redis"))
	require.Equal(t, "elasticache-redis", awsResourceType("elasticache-", "elasticache-redis"))
}

func TestResourceSubstitutionId(t *testing.T) {
	rs := &resourceSubstitution{App: "app1", RType: "rds-postgres", RName: "pg", StateId: "pg-ab12-app1", Tid: "ab12"}

	id := resourceSubstitutionId(rs)
	require.Equal(t, "##|app:app1|type:rds-postgres|resource:pg|stateId:pg-ab12-app1|tid:ab12|##", id)
	require.Equal(t, rs, parseResourceSubstitutionId(id))

	p := newTestAwsProvider()
	require.Equal(t, "pg-ab12-app1", p.resourceSubstitutionStateId(rs))

	legacy := parseResourceSubstitutionId("##|app:app1|type:rds-postgres|resource:pg|##")
	require.Equal(t, &resourceSubstitution{App: "app1", RType: "rds-postgres", RName: "pg"}, legacy)
	require.Equal(t, "pg-rrack1r-app1", p.resourceSubstitutionStateId(legacy))
}

// State logs are buffered per tenant and app: a shared key would flush one
// tenant's resource logs into another tenant's stream.
func TestStateLogStorageTenantScoping(t *testing.T) {
	s := &tempStateLogStorage{s: map[string][]string{}, threshold: 50}

	s.Add("ab12", "web", "one")
	s.Add("cd34", "web", "two")
	s.Add("", "web", "three")

	require.Equal(t, []string{"one"}, s.Get("ab12", "web"))
	require.Equal(t, []string{"two"}, s.Get("cd34", "web"))
	require.Equal(t, []string{"three"}, s.Get("", "web"))

	s.Reset("ab12", "web")
	require.Empty(t, s.Get("ab12", "web"))
	require.Equal(t, []string{"two"}, s.Get("cd34", "web"), "resetting one tenant must not clear another")

	// a bare tid+app concatenation would collide these two
	s.Add("ab", "12web", "collide")
	require.Empty(t, s.Get("ab12", "web"))
}
