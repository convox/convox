package k8s

import (
	"testing"

	"github.com/convox/convox/pkg/manifest"
	"github.com/convox/convox/pkg/options"
	"github.com/convox/convox/pkg/structs"
	"github.com/stretchr/testify/require"
)

func TestEnvironmentRackURL(t *testing.T) {
	const systemURL = "https://convox:pw@api.rack1-system.svc.cluster.local:5443" //nolint:gosec // test fixture, not a real credential

	tests := []struct {
		name    string
		disable bool
		gate    bool
		env     structs.Environment
		rack    string
		rackURL string
	}{
		{name: "default", rack: "rack1", rackURL: systemURL},
		{name: "disabled", disable: true, rack: "rack1"},
		{name: "gate", gate: true},
		{name: "gate and disabled", gate: true, disable: true},
		{name: "disabled with user value", disable: true, env: structs.Environment{"RACK_URL": "custom"}, rack: "rack1", rackURL: "custom"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := &Provider{
				Name:              "rack1",
				Namespace:         "rack1-system",
				Password:          "pw",
				AppRackURLDisable: tc.disable,
				FeatureGates:      map[string]bool{options.FeatureGateSystemEnvDisable: tc.gate},
			}

			env, err := p.environment(&structs.App{Name: "app1"}, &structs.Release{Id: "R1"}, manifest.Service{Name: "web"}, tc.env)
			require.NoError(t, err)

			rack, ok := env["RACK"]
			require.Equal(t, tc.rack != "", ok)
			require.Equal(t, tc.rack, rack)

			rackURL, ok := env["RACK_URL"]
			require.Equal(t, tc.rackURL != "", ok)
			require.Equal(t, tc.rackURL, rackURL)
		})
	}
}
