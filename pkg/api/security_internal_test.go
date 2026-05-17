package api

import (
	"testing"
)

func TestIsSafeProxyTarget(t *testing.T) {
	tests := []struct {
		host string
		safe bool
	}{
		// Blocked: empty
		{"", false},

		// Blocked: loopback IPv4
		{"127.0.0.1", false},
		{"127.0.0.2", false},
		{"127.255.255.255", false},

		// Blocked: loopback IPv6
		{"::1", false},
		{"[::1]", false},

		// Blocked: unspecified
		{"0.0.0.0", false},
		{"::", false},

		// Blocked: link-local
		{"169.254.1.1", false},
		{"169.254.169.254", false},
		{"fe80::1", false},

		// Blocked: IPv6 ULA (fc00::/7)
		{"fc00::1", false},
		{"fd00::1", false},

		// Blocked: IPv6 zone ID stripping
		{"::1%eth0", false},
		{"fe80::1%25lo", false},

		// Blocked: bracket-wrapped
		{"[127.0.0.1]", false},
		{"[fe80::1]", false},

		// Blocked: reserved hostnames
		{"localhost", false},
		{"LOCALHOST", false},
		{"Localhost", false},
		{"metadata.google.internal", false},
		{"metadata.google.internal.", false},
		{"METADATA.GOOGLE.INTERNAL", false},

		// Blocked: hex/octal IP notation (non-standard)
		{"0x7f000001", false},
		{"0X7F000001", false},
		{"017700000001", false},
		{"0177.0.0.1", false},

		// Allowed: public IPs
		{"8.8.8.8", true},
		{"1.1.1.1", true},
		{"203.0.113.1", true},

		// Allowed: public IPv6
		{"2001:db8::1", true},
		{"[2001:db8::1]", true},

		// Allowed: valid hostnames
		{"example.com", true},
		{"api.internal.svc.cluster.local", true},
		{"my-service", true},

		// Allowed: private RFC1918 (not blocked by this function)
		{"10.0.0.1", true},
		{"192.168.1.1", true},
		{"172.16.0.1", true},
	}

	for _, tt := range tests {
		t.Run(tt.host, func(t *testing.T) {
			got := isSafeProxyTarget(tt.host)
			if got != tt.safe {
				t.Errorf("isSafeProxyTarget(%q) = %v, want %v", tt.host, got, tt.safe)
			}
		})
	}
}

func TestIsNonStandardIP(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		// Hex notation
		{"0x7f000001", true},
		{"0X7F000001", true},
		{"0xDEADBEEF", true},

		// Octal/decimal-only strings (all digits and dots)
		{"017700000001", true},
		{"0177.0.0.1", true},
		{"123456", true},

		// Normal dotted-decimal (also matches: all digits+dots)
		{"127.0.0.1", true},
		{"10.0.0.1", true},

		// Not IPs: contain letters (non-hex-prefix)
		{"example.com", false},
		{"localhost", false},
		{"abc", false},
		{"10.0.0.1.nip.io", false},

		// Edge cases
		{"", false},
		{"0x", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := isNonStandardIP(tt.input)
			if got != tt.expected {
				t.Errorf("isNonStandardIP(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestSanitizeObjectKey(t *testing.T) {
	tests := []struct {
		key     string
		want    string
		wantErr bool
	}{
		// Valid keys
		{"foo/bar.txt", "foo/bar.txt", false},
		{"path/object1.ext", "path/object1.ext", false},
		{"simple.txt", "simple.txt", false},
		{"a/b/c/d.json", "a/b/c/d.json", false},

		// Path traversal attacks — blocked
		{"../../../etc/passwd", "", true},
		{"foo/../../../etc/shadow", "", true},
		// URL-encoded traversal is NOT decoded by sanitizeObjectKey —
		// stdapi/net/http decodes before the handler sees it, so literal
		// %2f in the key is not a traversal.
		{"..%2f..%2f..%2fetc/passwd", "..%2f..%2f..%2fetc/passwd", false},
		{"..", "", true},

		// Cleaned but valid (path.Clean normalizes)
		{"foo/./bar.txt", "foo/bar.txt", false},
		{"foo//bar.txt", "foo/bar.txt", false},

		// Empty/dot — blocked
		{"", "", true},
		{".", "", true},
		{"   ", "", true},

		// Leading slashes stripped
		{"/foo/bar.txt", "foo/bar.txt", false},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			got, err := sanitizeObjectKey(tt.key)
			if tt.wantErr {
				if err == nil {
					t.Errorf("sanitizeObjectKey(%q) = %q, want error", tt.key, got)
				}
			} else {
				if err != nil {
					t.Errorf("sanitizeObjectKey(%q) error: %v", tt.key, err)
				} else if got != tt.want {
					t.Errorf("sanitizeObjectKey(%q) = %q, want %q", tt.key, got, tt.want)
				}
			}
		})
	}
}
