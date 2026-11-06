package aws

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseInstanceID(t *testing.T) {
	tests := []struct {
		name       string
		providerID string
		expected   string
	}{
		{
			name:       "valid AWS provider ID",
			providerID: "aws:///us-east-1a/i-0123456789abcdef0",
			expected:   "i-0123456789abcdef0",
		},
		{
			name:       "empty string",
			providerID: "",
			expected:   "",
		},
		{
			name:       "non-AWS provider ID",
			providerID: "gce://my-project/us-central1-a/my-node",
			expected:   "",
		},
		{
			name:       "AWS prefix but no instance ID",
			providerID: "aws:///us-east-1a/vol-abc123",
			expected:   "",
		},
		{
			name:       "AWS prefix with trailing slash",
			providerID: "aws:///us-east-1a/",
			expected:   "",
		},
		{
			name:       "Azure provider ID",
			providerID: "azure:///subscriptions/sub/resourceGroups/rg/providers/Microsoft.Compute/virtualMachines/vm",
			expected:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseInstanceID(tt.providerID)
			require.Equal(t, tt.expected, result)
		})
	}
}
