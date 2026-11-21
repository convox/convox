package manifest_test

import (
	"fmt"
	"testing"

	"github.com/convox/convox/pkg/manifest"
	"github.com/stretchr/testify/require"
)

const kedaSqsTrigger = `        triggers:
        - type: aws-sqs-queue
          metadata: {queueURL: "http://q/", queueLength: "1", awsRegion: "us-east-1"}
`

const idleReplicaError = "scale.keda.idleReplicaCount must be lower than scale.min, and 0 when scale.min is 0 or 1"

func validateKedaScale(t *testing.T, scale string) (string, error) {
	t.Helper()
	m, err := manifest.Load([]byte("services:\n  worker:\n    build: .\n    scale:\n"+scale), map[string]string{})
	require.NoError(t, err)
	var verr error
	stderr := captureStderr(t, func() { verr = m.Validate() })
	return stderr, verr
}

func TestValidateKedaIdleReplicaCount(t *testing.T) {
	cases := []struct {
		min, idle int
		wantErr   bool
	}{
		{0, 0, false},
		{0, 1, true},
		{1, 0, false},
		{1, 1, true},
		{2, 1, false},
		{2, 2, true},
		{2, 0, false},
		{1, -1, true},
	}

	for _, tc := range cases {
		t.Run(fmt.Sprintf("min %d idle %d", tc.min, tc.idle), func(t *testing.T) {
			_, err := validateKedaScale(t, fmt.Sprintf("      count: %d-10\n      keda:\n        idleReplicaCount: %d\n%s", tc.min, tc.idle, kedaSqsTrigger))

			if tc.wantErr {
				require.ErrorContains(t, err, "service worker: "+idleReplicaError)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestValidateKedaIdleReplicaCountFixedCountPasses(t *testing.T) {
	_, err := validateKedaScale(t, "      count: 1\n      keda:\n        idleReplicaCount: 1\n"+kedaSqsTrigger)

	require.NoError(t, err)
}

func TestValidateKedaIdleReplicaCountWithAutoscaleTriggers(t *testing.T) {
	_, err := validateKedaScale(t, "      count: 1-3\n      autoscale:\n        cpu:\n          threshold: 70\n      keda:\n        idleReplicaCount: 1\n")

	require.ErrorContains(t, err, "service worker: "+idleReplicaError)
}

func TestValidateAutoscaleWithoutKedaPasses(t *testing.T) {
	_, err := validateKedaScale(t, "      count: 1-3\n      autoscale:\n        cpu:\n          threshold: 70\n")

	require.NoError(t, err)
}

func TestValidateWarnsKedaTriggersWithFixedCount(t *testing.T) {
	cases := []struct {
		name  string
		scale string
		fixed int
	}{
		{"count 2", "      count: 2\n      keda:\n" + kedaSqsTrigger, 2},
		{"count omitted", "      keda:\n" + kedaSqsTrigger, 1},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stderr, err := validateKedaScale(t, tc.scale)

			require.NoError(t, err)
			require.Contains(t, stderr, fmt.Sprintf(`WARNING: service "worker": scale.keda triggers are ignored because scale.count is fixed at %d; set a range such as 0-10 to let KEDA scale this service.`, tc.fixed))
		})
	}
}

func TestValidateNoKedaWarningForRange(t *testing.T) {
	stderr, err := validateKedaScale(t, "      count: 0-10\n      keda:\n"+kedaSqsTrigger)

	require.NoError(t, err)
	require.NotContains(t, stderr, "WARNING")
}

func TestValidateKedaWithAutoscaleAndFixedCount(t *testing.T) {
	stderr, err := validateKedaScale(t, "      count: 2\n      autoscale:\n        cpu:\n          threshold: 70\n      keda:\n"+kedaSqsTrigger)

	require.ErrorContains(t, err, "service worker: scale.max must be > scale.min when autoscale is enabled")
	require.NotContains(t, stderr, "scale.keda triggers are ignored")
}
