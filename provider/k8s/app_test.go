package k8s_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/convox/convox/pkg/atom"
	"github.com/convox/convox/pkg/options"
	"github.com/convox/convox/pkg/structs"
	"github.com/convox/convox/provider/k8s"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	ac "k8s.io/api/core/v1"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
)

func TestAppCancel(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		aa := p.Atom.(*atom.MockInterface)
		kk := p.Cluster.(*fake.Clientset)

		require.NoError(t, appCreate(kk, "rack1", "app1"))

		aa.On("Status", "rack1-app1", "app").Return("Updating", "R1234567", nil).Once()
		aa.On("Cancel", "rack1-app1", "app").Return(nil).Once()

		err := p.AppCancel("app1")
		require.NoError(t, err)
	})
}

func TestAppCancelMissingApp(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		err := p.AppCancel("app1")
		require.EqualError(t, err, "app not found: app1")
	})
}

func TestAppCancelError(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		aa := p.Atom.(*atom.MockInterface)
		kk := p.Cluster.(*fake.Clientset)

		require.NoError(t, appCreate(kk, "rack1", "app1"))

		aa.On("Status", "rack1-app1", "app").Return("Updating", "R1234567", nil).Once()
		aa.On("Cancel", "rack1-app1", "app").Return(fmt.Errorf("err1")).Once()

		err := p.AppCancel("app1")
		require.EqualError(t, err, "err1")
	})
}

func TestAppCancelInvalidState(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		aa := p.Atom.(*atom.MockInterface)
		kk := p.Cluster.(*fake.Clientset)

		require.NoError(t, appCreate(kk, "rack1", "app1"))

		aa.On("Status", "rack1-app1", "app").Return("Rollback", "R1234567", nil).Once()
		aa.On("Cancel", "rack1-app1", "app").Return(nil).Once()

		err := p.AppCancel("app1")
		require.NoError(t, err)
	})
}

func TestAppCreate(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		aa := p.Atom.(*atom.MockInterface)

		aa.On("Apply", "rack1-app1", "app", mock.Anything).Return(nil).Once().Run(func(args mock.Arguments) {
			cfg := args.Get(2).(*atom.ApplyConfig)
			requireYamlFixture(t, cfg.Template, "app.yml")
		})

		aa.On("Status", "rack1-app1", "app").Return("Updating", "R1234567", nil).Twice()

		a, err := p.AppCreate("app1", structs.AppCreateOptions{})
		require.NoError(t, err)
		require.NotNil(t, a)

		assert.Equal(t, "3", a.Generation)
		assert.Equal(t, "app1", a.Name)
	})
}

func TestAppDelete(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		aa := p.Atom.(*atom.MockInterface)
		kk := p.Cluster.(*fake.Clientset)

		require.NoError(t, appCreate(kk, "rack1", "app1"))

		aa.On("Status", "rack1-app1", "app").Return("Updating", "R1234567", nil).Once()

		err := p.AppDelete("app1")
		require.NoError(t, err)

		_, err = kk.CoreV1().Namespaces().Get(context.TODO(), "rack1-app1", am.GetOptions{})
		require.EqualError(t, err, `namespaces "rack1-app1" not found`)
	})
}

func TestAppDeleteMissingApp(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		err := p.AppDelete("app1")
		require.EqualError(t, err, "app not found: app1")
	})
}

func TestNamespaceApp(t *testing.T) {
	tests := []struct {
		Name      string
		RackName  string
		AppName   string
		Namespace string
	}{
		{
			Name:      "Success",
			RackName:  "rack1",
			AppName:   "app1",
			Namespace: "rack1-app1",
		},
		{
			Name:      "Namespace not found",
			RackName:  "rack2",
			AppName:   "app2",
			Namespace: "app2",
		},
	}

	testProvider(t, func(p *k8s.Provider) {
		for _, test := range tests {
			fn := func(t *testing.T) {
				kk := p.Cluster.(*fake.Clientset)

				require.NoError(t, appCreate(kk, test.RackName, test.AppName))

				ns, err := p.NamespaceApp(test.Namespace)

				if err != nil {
					require.EqualError(t, err, fmt.Sprintf("namespaces %q not found", test.Namespace))
				} else {
					require.NoError(t, err)
					require.Equal(t, ns, test.AppName)
				}
			}

			t.Run(test.Name, fn)
		}
	})
}

func TestAppGet(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		aa := p.Atom.(*atom.MockInterface)
		kk := p.Cluster.(*fake.Clientset)

		aa.On("Status", "rack1-app1", "app").Return("Running", "R1234567", nil).Once()

		require.NoError(t, appCreate(kk, "rack1", "app1"))

		a, err := p.AppGet("app1")
		require.NoError(t, err)

		assert.Equal(t, "3", a.Generation)
		assert.Equal(t, false, a.Locked)
		assert.Equal(t, "app1", a.Name)
		assert.Equal(t, "R1234567", a.Release)
		assert.Equal(t, "", a.Router)
		assert.Equal(t, "running", a.Status)
	})
}

func TestAppGetMissing(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		a, err := p.AppGet("app1")
		require.EqualError(t, err, "app not found: app1")
		require.Nil(t, a)
	})
}

func TestAppGetUpdating(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		aa := p.Atom.(*atom.MockInterface)
		kk := p.Cluster.(*fake.Clientset)

		aa.On("Status", "rack1-app1", "app").Return("Updating", "", nil).Once()

		ns := &ac.Namespace{
			ObjectMeta: am.ObjectMeta{
				Name: "rack1-app1",
			},
		}
		_, err := kk.CoreV1().Namespaces().Create(context.TODO(), ns, am.CreateOptions{})
		require.NoError(t, err)

		a, err := p.AppGet("app1")
		require.NoError(t, err)
		require.Equal(t, "updating", a.Status)
	})
}

func TestAppList(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		aa := p.Atom.(*atom.MockInterface)
		kk := p.Cluster.(*fake.Clientset)

		aa.On("Status", "rack1-app1", "app").Return("Running", "R1234567", nil).Once()
		aa.On("Status", "rack1-app2", "app").Return("Updating", "R2345678", nil).Once()

		require.NoError(t, appCreate(kk, "rack1", "app1"))
		require.NoError(t, appCreate(kk, "rack1", "app2"))

		as, err := p.AppList()
		require.NoError(t, err)
		require.Equal(t, 2, len(as))

		assert.Equal(t, "3", as[0].Generation)
		assert.Equal(t, false, as[0].Locked)
		assert.Equal(t, "app1", as[0].Name)
		assert.Equal(t, "R1234567", as[0].Release)
		assert.Equal(t, "", as[0].Router)
		assert.Equal(t, "running", as[0].Status)

		assert.Equal(t, "3", as[1].Generation)
		assert.Equal(t, false, as[1].Locked)
		assert.Equal(t, "app2", as[1].Name)
		assert.Equal(t, "R2345678", as[1].Release)
		assert.Equal(t, "", as[1].Router)
		assert.Equal(t, "updating", as[1].Status)
	})
}

func TestAppLogs(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		r, err := p.AppLogs("app1", structs.LogsOptions{})
		require.EqualError(t, err, "unimplemented")
		require.Nil(t, r)
	})
}

func TestAppMetrics(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		ms, err := p.AppMetrics("app1", structs.MetricsOptions{})
		require.EqualError(t, err, "unimplemented")
		require.Nil(t, ms)
	})
}

func TestAppNamespace(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		ns := p.AppNamespace("app1")
		require.Equal(t, "rack1-app1", ns)
	})
}

func TestAppUpdateLocked(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		aa := p.Atom.(*atom.MockInterface)
		kk := p.Cluster.(*fake.Clientset)

		aa.On("Apply", "rack1-app1", "app", mock.Anything).Return(nil).Once().Run(func(args mock.Arguments) {
			cfg := args.Get(2).(*atom.ApplyConfig)
			requireYamlFixture(t, cfg.Template, "app-locked.yml")
		})

		aa.On("Status", "rack1-app1", "app").Return("Running", "", nil).Twice()

		require.NoError(t, appCreate(kk, "rack1", "app1"))

		err := p.AppUpdate("app1", structs.AppUpdateOptions{Lock: options.Bool(true)})
		require.NoError(t, err)

		ns, err := p.Cluster.CoreV1().Namespaces().Get(context.TODO(), "rack1-app1", am.GetOptions{})
		require.NoError(t, err)
		require.Equal(t, "true", ns.Annotations["convox.com/lock"])
	})
}

func TestAppUpdateDoesNotOverwriteExisting(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		aa := p.Atom.(*atom.MockInterface)
		kk := p.Cluster.(*fake.Clientset)

		aa.On("Apply", "rack1-app1", "app", mock.Anything).Return(nil).Once().Run(func(args mock.Arguments) {
			cfg := args.Get(2).(*atom.ApplyConfig)
			requireYamlFixture(t, cfg.Template, "app-locked.yml")
		}).Once()

		aa.On("Apply", "rack1-app1", "app", mock.Anything).Return(nil).Once().Run(func(args mock.Arguments) {
			cfg := args.Get(2).(*atom.ApplyConfig)
			requireYamlFixture(t, cfg.Template, "app-locked-params.yml")
		}).Once()

		aa.On("Status", "rack1-app1", "app").Return("Running", "", nil).Times(4)

		require.NoError(t, appCreate(kk, "rack1", "app1"))

		err := p.AppUpdate("app1", structs.AppUpdateOptions{Lock: options.Bool(true)})
		require.NoError(t, err)

		err = p.AppUpdate("app1", structs.AppUpdateOptions{Parameters: map[string]string{"Test": "bar"}})
		require.NoError(t, err)

		ns, err := p.Cluster.CoreV1().Namespaces().Get(context.TODO(), "rack1-app1", am.GetOptions{})
		require.NoError(t, err)
		require.Equal(t, "true", ns.Annotations["convox.com/lock"])
		require.Equal(t, `{"Test":"bar"}`, ns.Annotations["convox.com/params"])
	})
}

func TestAppUpdateParameters(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		aa := p.Atom.(*atom.MockInterface)
		kk := p.Cluster.(*fake.Clientset)

		aa.On("Apply", "rack1-app1", "app", mock.Anything).Return(nil).Once().Run(func(args mock.Arguments) {
			cfg := args.Get(2).(*atom.ApplyConfig)
			requireYamlFixture(t, cfg.Template, "app-params.yml")
		})

		aa.On("Status", "rack1-app1", "app").Return("Running", "", nil).Twice()

		require.NoError(t, appCreate(kk, "rack1", "app1"))

		err := p.AppUpdate("app1", structs.AppUpdateOptions{Parameters: map[string]string{"Test": "bar"}})
		require.NoError(t, err)

		ns, err := p.Cluster.CoreV1().Namespaces().Get(context.Background(), "rack1-app1", am.GetOptions{})
		require.NoError(t, err)
		require.Equal(t, `{"Test":"bar"}`, ns.Annotations["convox.com/params"])
	})
}

func TestAppUpdateExistingRelease(t *testing.T) {
	t.Skip("implement after testing releases")
}

func TestAppUpdateMissing(t *testing.T) {
	testProvider(t, func(p *k8s.Provider) {
		err := p.AppUpdate("app1", structs.AppUpdateOptions{Lock: options.Bool(true)})
		require.EqualError(t, err, "app not found: app1")
	})
}

func appCreate(c kubernetes.Interface, rack, name string) error {
	_, err := c.CoreV1().Namespaces().Create(
		context.TODO(),
		&ac.Namespace{
			ObjectMeta: am.ObjectMeta{
				Name:        fmt.Sprintf("%s-%s", rack, name),
				Annotations: map[string]string{"convox.com/lock": "false"},
				Labels: map[string]string{
					"app":    name,
					"name":   name,
					"rack":   rack,
					"system": "convox",
					"type":   "app",
				},
			},
		},
		am.CreateOptions{},
	)

	return errors.WithStack(err)
}
