package aws_test

import (
	"testing"

	"github.com/convox/convox/provider/aws"
	"github.com/stretchr/testify/assert"
)

func TestDeploymentTimeout(t *testing.T) {
	testProvider(t, func(p *aws.Provider) {
		assert.Equal(t, 1800, p.DeploymentTimeout())
	})
}
