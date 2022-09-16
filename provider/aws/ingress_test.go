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
	testProvider(t, func(p *aws.Provider) {
		ann, err := p.IngressAnnotations("")
		require.NoError(t, err)

		annotations := map[string]string{"cert-manager.io/cluster-issuer": "letsencrypt"}
		assert.Equal(t, annotations, ann)
	})
}
