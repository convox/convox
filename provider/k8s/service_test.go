package k8s_test

import (
	"testing"

	"github.com/convox/convox/pkg/atom"
	"github.com/convox/convox/pkg/manifest"
	"github.com/convox/convox/provider/k8s"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/kubernetes/fake"
)

func TestServiceHost(t *testing.T) {
	tests := []struct {
		Name         string
		AppName      string
		Service      manifest.Service
		HostResponse string
	}{
		{
			Name:    "Internal",
			AppName: "app1",
			Service: manifest.Service{
				Name:     "app1-service",
				Internal: true,
			},
			HostResponse: "app1-service.app1.rack1.local",
		},
		{
			Name:    "Internal Router",
			AppName: "app1",
			Service: manifest.Service{
				Name:           "app1-service",
				InternalRouter: true,
			},
			HostResponse: "app1-service.app1.domain-internal",
		},
		{
			Name:    "Public",
			AppName: "app2",
			Service: manifest.Service{
				Name:     "app2-service",
				Internal: false,
			},
			HostResponse: "app2-service.app2.domain1",
		},
	}

	testProvider(t, func(p *k8s.Provider) {
		for _, test := range tests {
			fn := func(t *testing.T) {
				host := p.ServiceHost(test.AppName, &test.Service)
				assert.Equal(t, host, test.HostResponse)
			}

			t.Run(test.Name, fn)
		}
	})
}

func TestServiceList(t *testing.T) {
	tests := []struct {
		Name      string
		AppName   string
		RackName  string
		Namespace string
		CreateApp bool
		Release   string
		Err       error
	}{
		{
			Name:      "Success",
			AppName:   "app",
			Namespace: "rack1-app",
			RackName:  "rack1",
			CreateApp: true,
			Release:   "r1234567",
			Err:       nil,
		},
		{
			Name:      "App not found",
			AppName:   "app2",
			Namespace: "rack2-app2",
			CreateApp: false,
			Err:       errors.New("app not found: app2"),
		},
	}

	testProvider(t, func(p *k8s.Provider) {
		for _, test := range tests {
			fn := func(t *testing.T) {
				aa := p.Atom.(*atom.MockInterface)

				if test.CreateApp {
					kk := p.Cluster.(*fake.Clientset)
					aa.On("Status", test.Namespace, test.AppName).Return("Running", test.Release, nil).Once()

					require.NoError(t, appCreate(kk, test.RackName, test.AppName))
					require.NoError(t, releaseCreate(p.Convox, test.Namespace, test.Release, "basic"))
				}

				_, err := p.ServiceList(test.AppName)
				if err != nil {
					assert.Equal(t, test.Err.Error(), err.Error())
				} else {
					require.NoError(t, err)
				}
			}

			t.Run(test.Name, fn)
		}
	})
}
