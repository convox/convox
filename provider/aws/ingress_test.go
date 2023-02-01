package aws_test

import (
	"testing"

	"github.com/convox/convox/provider/aws"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIngressClass(t *testing.T) {
	testProvider(t, func(p *aws.Provider) {
		assert.Equal(t, "nginx", p.IngressClass())
	})
}

func TestIngressAnnotations(t *testing.T) {
	tests := []struct {
		Name        string
		Duration    string
		Annotations map[string]string
	}{
		{
			Name:        "Not passing duration",
			Duration:    "",
			Annotations: map[string]string{"cert-manager.io/cluster-issuer": "letsencrypt"},
		},
		{
			Name:     "Passing duration",
			Duration: "720h",
			Annotations: map[string]string{
				"cert-manager.io/cluster-issuer": "letsencrypt",
				"cert-manager.io/duration":       "720h",
			},
		},
	}

	for _, test := range tests {
		fn := func(t *testing.T) {
			testProvider(t, func(p *aws.Provider) {
				ann, err := p.IngressAnnotations(test.Duration)
				require.NoError(t, err)
				assert.Equal(t, test.Annotations, ann)
			})
		}
		t.Run(test.Name, fn)
	}
}
