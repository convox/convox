package aws_test

import (
	"errors"
	"testing"

	"github.com/convox/convox/provider/aws"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegistryAuth(t *testing.T) {
	tests := []struct {
		Name     string
		Host     string
		Username string
		Password string
		Err      error
	}{
		{
			Name:     "error on auth registry",
			Host:     "134537970938.dkr.ecr.us-east-1.amazonaws.com/dev-remote/httpd",
			Username: "testusername",
			Password: "testpassword",
			Err:      errors.New("unable to authenticate with ecr registry: 134537970938.dkr.ecr.us-east-1.amazonaws.com/dev-remote/httpd"),
		},
		{
			Name:     "Host don't match",
			Host:     "wronghost.com.br",
			Username: "testusername",
			Password: "testpassword",
			Err:      nil,
		},
	}

	testProvider(t, func(p *aws.Provider) {
		for _, test := range tests {
			fn := func(t *testing.T) {
				username, password, err := p.RegistryAuth(test.Host, test.Username, test.Password)
				if err == nil {
					require.NoError(t, err)
					assert.Equal(t, username, test.Username)
					assert.Equal(t, password, test.Password)
				} else {
					assert.Equal(t, test.Err.Error(), err.Error())
				}
			}

			t.Run(test.Name, fn)
		}
	})
}
