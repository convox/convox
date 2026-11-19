package cli

import (
	"strings"
	"testing"
)

func TestValidateImdsHopLimit(t *testing.T) {
	const (
		needsAgent = "requires pod_identity_agent_enable=true"
		agentOff   = "must be set in a call of its own"
	)

	kv := func(pairs ...string) map[string]string {
		m := map[string]string{}
		for _, p := range pairs {
			parts := strings.SplitN(p, "=", 2)
			m[parts[0]] = parts[1]
		}
		return m
	}

	hop, required, agent := "imds_http_hop_limit=1", "imds_http_tokens=required", "pod_identity_agent_enable=true"
	noAgent := "pod_identity_agent_enable=false"

	tests := []struct {
		name    string
		current map[string]string
		params  map[string]string
		force   bool
		wantErr string
	}{
		{"hop limit 2", kv(), kv("imds_http_hop_limit=2"), false, ""},
		{"hop limit 1 with optional tokens", kv(), kv(hop, "imds_http_tokens=optional"), false, ""},
		{"hop limit 1 alone", kv(), kv(hop), false, ""},
		{"required tokens alone", kv(), kv(required), false, ""},

		{"agent stored, both set", kv(agent), kv(hop, required), false, ""},
		{"agent and tokens stored, hop set", kv(agent, required), kv(hop), false, ""},
		{"agent and hop stored, tokens set", kv(agent, hop), kv(required), false, ""},
		{"restricted with agent, set again", kv(agent, hop, required), kv(hop, agent), false, ""},

		{"agent in the same call", kv(), kv(hop, required, agent), false, needsAgent},
		{"agent in the same call, tokens stored", kv(required), kv(hop, agent), false, needsAgent},
		{"no agent", kv(), kv(hop, required), false, needsAgent},
		{"agent stored false", kv(noAgent), kv(hop, required), false, needsAgent},
		{"tokens stored, hop set", kv(required), kv(hop), false, needsAgent},
		{"already restricted, tokens set again", kv(hop, required), kv(required), false, needsAgent},

		{"repair", kv(hop, required), kv(agent), false, ""},
		{"already restricted, unrelated param", kv(hop, required), kv("node_disk=50"), false, ""},
		{"already restricted, hop raised", kv(hop, required), kv("imds_http_hop_limit=3"), false, ""},
		{"restricted with agent, unrelated param", kv(agent, hop, required), kv("node_disk=50"), false, ""},
		{"restricted with agent, hop raised", kv(agent, hop, required), kv("imds_http_hop_limit=3"), false, ""},

		{"agent off while restricted", kv(agent, hop, required), kv(noAgent), false, agentOff},
		{"agent off in the restricting call", kv(agent), kv(hop, required, noAgent), false, agentOff},
		{"agent off with hop raised", kv(agent, hop, required), kv("imds_http_hop_limit=3", noAgent), false, agentOff},
		{"agent off with another param", kv(agent), kv("node_disk=50", noAgent), false, agentOff},

		{"agent off alone", kv(agent), kv(noAgent), false, ""},
		{"agent never on, set false with another param", kv(), kv("node_disk=50", noAgent), false, ""},

		{"force, no agent", kv(), kv(hop, required), true, ""},
		{"force, agent off while restricted", kv(agent, hop, required), kv(noAgent), true, ""},

		{"hop limit 01", kv(), kv("imds_http_hop_limit=01", required), false, needsAgent},
		{"hop limit +1", kv(), kv("imds_http_hop_limit=+1", required), false, needsAgent},
		{"hop limit 1.0", kv(), kv("imds_http_hop_limit=1.0", required), false, needsAgent},
		{"hop limit 1e0", kv(), kv("imds_http_hop_limit=1e0", required), false, needsAgent},
		{"stored hop limit 1.0", kv("imds_http_hop_limit=1.0"), kv(required), false, needsAgent},
		{"stored agent 1", kv("pod_identity_agent_enable=1"), kv(hop, required), false, ""},
		{"stored agent 1, turned off while restricted", kv("pod_identity_agent_enable=1", hop, required), kv(noAgent), false, agentOff},
		{"stored agent null", kv("pod_identity_agent_enable=null"), kv(hop, required), false, needsAgent},
		{"stored hop limit null", kv("imds_http_hop_limit=null", required, agent), kv(noAgent), false, ""},
		{"nil stored params", nil, kv("node_disk=50"), false, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateAndMutateParams(tt.params, "aws", tt.current, tt.force)
			switch {
			case tt.wantErr == "" && err != nil:
				t.Fatalf("rejected: %v", err)
			case tt.wantErr != "" && err == nil:
				t.Fatalf("accepted, want error containing %q", tt.wantErr)
			case tt.wantErr != "" && !strings.Contains(err.Error(), tt.wantErr):
				t.Fatalf("error %q does not contain %q", err, tt.wantErr)
			}
		})
	}
}
