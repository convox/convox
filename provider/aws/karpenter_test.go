package aws

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseInstanceID(t *testing.T) {
	tests := []struct {
		name       string
		providerID string
		want       string
	}{
		{"standard EKS format", "aws:///us-east-1a/i-0123456789abcdef0", "i-0123456789abcdef0"},
		{"empty", "", ""},
		{"no instance prefix", "aws:///us-east-1a/vol-123", ""},
		{"just instance ID", "i-abc123", ""},
		{"different region", "aws:///eu-west-1c/i-fffff", "i-fffff"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseInstanceID(tt.providerID)
			require.Equal(t, tt.want, got)
		})
	}
}
