package k8s

import "testing"

func TestIsAllowedTarFlag(t *testing.T) {
	tests := []struct {
		flag    string
		allowed bool
	}{
		// Allowed flags
		{"--no-same-owner", true},
		{"--no-same-permissions", true},
		{"--strip-components=1", true},
		{"--strip-components=0", true},
		{"--strip-components=42", true},

		// Blocked: command injection via tar flags
		{"--checkpoint-action=exec=sh", false},
		{"--to-command=sh", false},
		{"--use-compress-program=evil", false},
		{"--transform=s/^/owned/", false},
		{"--warning=no-timestamp", false},

		// Blocked: dangerous flags
		{"-C", false},
		{"--directory=/tmp", false},
		{"--exclude=*", false},
		{"-xvf", false},

		// Blocked: empty
		{"", false},

		// Blocked: strip-components with non-numeric value
		{"--strip-components=abc", false},
		{"--strip-components=1;echo pwned", false},
		{"--strip-components=", false},
		{"--strip-components=1a", false},

		// Blocked: similar but not exact match
		{"--no-same-owner=yes", false},
		{"--no-same-ownerx", false},
	}

	for _, tt := range tests {
		t.Run(tt.flag, func(t *testing.T) {
			got := isAllowedTarFlag(tt.flag)
			if got != tt.allowed {
				t.Errorf("isAllowedTarFlag(%q) = %v, want %v", tt.flag, got, tt.allowed)
			}
		})
	}
}
