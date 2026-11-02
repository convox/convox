package k8s_test

import (
	"testing"

	"github.com/convox/convox/pkg/atom"
	"github.com/convox/convox/pkg/options"
	"github.com/convox/convox/pkg/structs"
	"github.com/convox/convox/provider/k8s"
	ca "github.com/convox/convox/provider/k8s/pkg/apis/convox/v1"
	cvfake "github.com/convox/convox/provider/k8s/pkg/client/clientset/versioned/fake"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func promoteIdentifiers(t *testing.T, manifest, env string, repromote, force bool) (bool, error) {
	t.Helper()

	var applied bool
	var perr error

	testProvider(t, func(p *k8s.Provider) {
		kk, ok := p.Cluster.(*fake.Clientset)
		require.True(t, ok)
		require.NoError(t, appCreate(kk, "rack1", "app1"))

		cc, ok := p.Convox.(*cvfake.Clientset)
		require.True(t, ok)
		require.NoError(t, releaseCreateInline(cc, "rack1-app1", "release0", "services:\n  web:\n    image: docker.io/library/nginx\n    port: 5000\n"))
		_, err := cc.ConvoxV1().Releases("rack1-app1").Create(&ca.Release{
			ObjectMeta: am.ObjectMeta{Name: "release1"},
			Spec:       ca.ReleaseSpec{Build: "build1", Env: env, Manifest: manifest, Created: "20200101.000000.000000000"},
		})
		require.NoError(t, err)
		require.NoError(t, buildCreate(cc, "rack1-app1", "build1", "basic"))

		current := "release0"
		if repromote {
			current = "release1"
		}

		aa, ok := p.Atom.(*atom.MockInterface)
		require.True(t, ok)
		aa.On("Status", "rack1-app1", "app").Return("Running", current, nil)
		aa.On("Apply", "rack1-app1", "app", mock.Anything).Return(nil).Maybe().Run(func(mock.Arguments) {
			applied = true
		})

		perr = p.ReleasePromote("app1", "release1", structs.ReleasePromoteOptions{Force: options.Bool(force)})
	})

	return applied, perr
}

func TestReleasePromoteIdentifiers(t *testing.T) {
	const svc = "    image: docker.io/library/nginx\n    port: 5000\n"

	tests := []struct {
		Name     string
		Manifest string
		Env      string
		Err      string
	}{
		{"ServiceName", "services:\n  \"web\\n---\\nkind: ClusterRole\":\n" + svc, "", `invalid service name: "web\n---\nkind: ClusterRole"`},
		{"TimerName", "services:\n  web:\n" + svc + "timers:\n  \"t\\nx: y\":\n    command: x\n    schedule: \"0 * * * *\"\n    service: web\n", "", `invalid timer name: "t\nx: y"`},
		{"VolumeId", "services:\n  web:\n" + svc + "    volumeOptions:\n      - emptyDir:\n          id: \"a\\n---\\nb\"\n          mountPath: /x\n", "", `invalid volume id: "a\n---\nb"`},
		{"InitConfigMountId", "services:\n  web:\n" + svc + "    initContainer:\n      configMounts:\n        - id: \"c d\"\n          dir: /etc\n          filename: f\n", "", `invalid config mount id: "c d"`},
		{"RdsType", "resources:\n  db:\n    type: \"rds-postgres\\n---\\nkind: ClusterRole\"\nservices:\n  web:\n" + svc, "", `invalid resource type: "rds-postgres\n---\nkind: ClusterRole"`},
		{"ElastiCacheType", "resources:\n  c:\n    type: \"elasticache-redis\\n#|type:x\"\nservices:\n  web:\n" + svc, "", `invalid resource type: "elasticache-redis\n#|type:x"`},
		{"BalancerName", "services:\n  web:\n" + svc + "balancers:\n  \"b\\n---\\nx\":\n    service: web\n    ports:\n      80: 5000\n", "", `invalid balancer name: "b\n---\nx"`},
		{"ResourceName", "resources:\n  \"d b\":\n    type: postgres\nservices:\n  web:\n" + svc, "", `invalid resource name: "d b"`},
		{"ServiceConfigMountId", "services:\n  web:\n" + svc + "    configMounts:\n      - id: \"c d\"\n        dir: /etc\n        filename: f\n", "", `invalid config mount id: "c d"`},
		{"EfsVolumeId", "services:\n  web:\n" + svc + "    volumeOptions:\n      - awsEfs:\n          id: \"e f\"\n          mountPath: /x\n          accessMode: ReadWriteMany\n", "", `invalid volume id: "e f"`},
		{"InitPvcVolumeId", "services:\n  web:\n" + svc + "    initContainer:\n      volumeOptions:\n        - persistentVolumeClaim:\n            id: \"p q\"\n            mountPath: /b\n            size: 1Gi\n", "", `invalid volume id: "p q"`},
		{"MultiTypeVolumeOption", "services:\n  web:\n" + svc + "    volumeOptions:\n      - emptyDir:\n          id: a\n          mountPath: /a\n        awsEfs:\n          id: \"e f\"\n          mountPath: /b\n          accessMode: ReadWriteMany\n", "", `invalid volume id: "e f"`},
		{"InitMultiTypeVolumeOption", "services:\n  web:\n" + svc + "    initContainer:\n      volumeOptions:\n        - emptyDir:\n            id: a\n            mountPath: /a\n          persistentVolumeClaim:\n            id: \"p q\"\n            mountPath: /b\n            size: 1Gi\n", "", `invalid volume id: "p q"`},
		{"InitAzureFilesVolumeId", "services:\n  web:\n" + svc + "    initContainer:\n      volumeOptions:\n        - emptyDir:\n            id: a\n            mountPath: /a\n          azureFiles:\n            id: \"z z\"\n            mountPath: /c\n            accessMode: ReadWriteMany\n", "", `invalid volume id: "z z"`},
		{"MaskedTypeAllowed", "resources:\n  database:\n    type: postgres:13\nservices:\n  web:\n" + svc, "DATABASE_URL=postgres://u:p@h:5432/db", ""},
		{"Valid", "services:\n  web.v2:\n" + svc, "", ""},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			for _, force := range []bool{false, true} {
				applied, err := promoteIdentifiers(t, test.Manifest, test.Env, false, force)
				if test.Err == "" {
					require.NoError(t, err)
					require.True(t, applied)
					continue
				}
				require.EqualError(t, err, test.Err)
				require.False(t, applied)
			}
		})
	}
}

func TestReleasePromoteIdentifiersRepromote(t *testing.T) {
	applied, err := promoteIdentifiers(t, "services:\n  web:\n    image: docker.io/library/nginx\n    port: 5000\n", "", true, false)
	require.NoError(t, err)
	require.True(t, applied)
}
