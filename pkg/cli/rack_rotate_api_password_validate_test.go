package cli

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateRotateAPIPassword(t *testing.T) {
	for provider := range providerKnownParams {
		for _, force := range []bool{false, true} {
			require.NoError(t, validateAndMutateParams(map[string]string{"rotate_api_password": "true"}, provider, nil, force), provider)

			for _, v := range []string{"yes", "false", "1", "TRUE"} {
				err := validateAndMutateParams(map[string]string{"rotate_api_password": v}, provider, nil, force)
				require.EqualError(t, err, "param 'rotate_api_password' only accepts true", "%s %s force=%t", provider, v, force)
			}
		}
	}
}
