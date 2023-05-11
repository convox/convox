package aws_test

import (
	"errors"
	"testing"

	"github.com/convox/convox/pkg/atom"
	"github.com/convox/convox/provider/aws"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/kubernetes/fake"
)

func TestRepositoryAuth(t *testing.T) {
	tests := []struct {
		Name      string
		Appname   string
		Namespace string
		Err       error
	}{
		{
			Name:      "Error Auth registry",
			Appname:   "app1",
			Namespace: "rack1-app1",
			Err:       errors.New("unable to authenticate with ecr registry: 134537970938.dkr.ecr.us-east-1.amazonaws.com/dev-remote/httpd"),
		},
	}

	testProvider(t, func(p *aws.Provider) {
		for _, test := range tests {
			fn := func(t *testing.T) {
				kk := p.Provider.Cluster.(*fake.Clientset)
				require.NoError(t, appCreate(kk, "rack1", "app1"))

				if test.Err == nil {
					aa := p.Atom.(*atom.MockInterface)
					aa.On("Status", test.Namespace, "app").Return("Updating", "R1234567", nil).Once()
				}

				host, _, err := p.RepositoryAuth(test.Appname)
				if err == nil {
					require.NoError(t, err)
					assert.Equal(t, host, "AWS")
				} else {
					assert.Equal(t, test.Err.Error(), err.Error())
				}
			}

			t.Run(test.Name, fn)
		}
	})
}
