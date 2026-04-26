package manifest_test

import (
	"os"
	"testing"

	"github.com/convox/convox/pkg/manifest"
	"github.com/stretchr/testify/require"
)

// loadFixture reads a testdata YAML and runs Load + Validate. Returns
// the validate error (or nil on success). Empty env so environment
// interpolation cannot mask manifest content.
func loadFixture(t *testing.T, name string) error {
	t.Helper()
	data, err := os.ReadFile("testdata/" + name)
	require.NoError(t, err, "read fixture %s", name)
	m, err := manifest.Load(data, map[string]string{})
	if err != nil {
		return err
	}
	return m.Validate()
}

// TestParseAutoShutdownManifest_FullSchema_Succeeds verifies the full
// fixture parses with all 5 new fields populated. Per Set G v2 §2.2.
func TestParseAutoShutdownManifest_FullSchema_Succeeds(t *testing.T) {
	data, err := os.ReadFile("testdata/auto-shutdown-full.yml")
	require.NoError(t, err)
	m, err := manifest.Load(data, map[string]string{})
	require.NoError(t, err)
	require.NoError(t, m.Validate())
	require.Equal(t, "auto-shutdown", m.Budget.AtCapAction)
	require.Equal(t, "https://hooks.example.com/budget", m.Budget.AtCapWebhookUrl)
	require.Equal(t, []string{"api"}, m.Budget.NeverAutoShutdown)
	require.Equal(t, "largest-cost", m.Budget.ShutdownOrder)
	require.Equal(t, 30, m.Budget.NotifyBeforeMinutes)
	require.Equal(t, "5m", m.Budget.ShutdownGracePeriod)
	require.Equal(t, "auto-on-reset", m.Budget.RecoveryMode)
}

// TestParseAutoShutdownManifest_MinimalSchema_Succeeds verifies the
// minimal fixture (3 required fields, the rest zero/default).
func TestParseAutoShutdownManifest_MinimalSchema_Succeeds(t *testing.T) {
	require.NoError(t, loadFixture(t, "auto-shutdown-minimal.yml"))
}

// TestParseAutoShutdownManifest_ProtectedSchema_Succeeds verifies the
// neverAutoShutdown-only fixture.
func TestParseAutoShutdownManifest_ProtectedSchema_Succeeds(t *testing.T) {
	require.NoError(t, loadFixture(t, "auto-shutdown-protected.yml"))
}

// TestParseAutoShutdownManifest_NoWebhookUrl_Rule1_RejectsHardFail
// verifies rule 1: missing atCapWebhookUrl rejects.
func TestParseAutoShutdownManifest_NoWebhookUrl_Rule1_RejectsHardFail(t *testing.T) {
	err := loadFixture(t, "auto-shutdown-invalid-rule1.yml")
	require.Error(t, err)
	require.Contains(t, err.Error(), "requires budget.atCapWebhookUrl")
}

// TestParseAutoShutdownManifest_NoMonthlyCap_Rule2_RejectsHardFail
// verifies rule 2: missing monthlyCapUsd rejects.
func TestParseAutoShutdownManifest_NoMonthlyCap_Rule2_RejectsHardFail(t *testing.T) {
	err := loadFixture(t, "auto-shutdown-invalid-rule2.yml")
	require.Error(t, err)
	require.Contains(t, err.Error(), "requires budget.monthlyCapUsd")
}

// TestParseAutoShutdownManifest_NotifyBeforeMinutesOutOfRange_Rule4_RejectsHardFail
// verifies rule 4: notifyBeforeMinutes out of [5, 1440] rejects.
func TestParseAutoShutdownManifest_NotifyBeforeMinutesOutOfRange_Rule4_RejectsHardFail(t *testing.T) {
	err := loadFixture(t, "auto-shutdown-invalid-rule4.yml")
	require.Error(t, err)
	require.Contains(t, err.Error(), "must be between 5 and 1440")
}

// TestParseAutoShutdownManifest_ShutdownGracePeriodOutOfRange_Rule5_RejectsHardFail
// verifies rule 5: shutdownGracePeriod outside [0s, 1h] rejects.
func TestParseAutoShutdownManifest_ShutdownGracePeriodOutOfRange_Rule5_RejectsHardFail(t *testing.T) {
	err := loadFixture(t, "auto-shutdown-invalid-rule5.yml")
	require.Error(t, err)
	require.Contains(t, err.Error(), "must be between 0s and 1h")
}

// TestParseAutoShutdownManifest_ShutdownGracePeriodUnparseable_Rule6_RejectsHardFail
// verifies rule 6: unparseable duration rejects.
func TestParseAutoShutdownManifest_ShutdownGracePeriodUnparseable_Rule6_RejectsHardFail(t *testing.T) {
	err := loadFixture(t, "auto-shutdown-invalid-rule6.yml")
	require.Error(t, err)
	require.Contains(t, err.Error(), "must be a Go duration string")
}

// TestParseAutoShutdownManifest_ShutdownOrderInvalid_Rule7_RejectsHardFail
// verifies rule 7: invalid enum rejects.
func TestParseAutoShutdownManifest_ShutdownOrderInvalid_Rule7_RejectsHardFail(t *testing.T) {
	err := loadFixture(t, "auto-shutdown-invalid-rule7.yml")
	require.Error(t, err)
	require.Contains(t, err.Error(), "must be one of")
}

// TestParseAutoShutdownManifest_ShutdownOrderPriorityAnnotation_Rule7a_RejectsWithDeferralMessage
// verifies rule 7a: priority-annotation deferral message.
func TestParseAutoShutdownManifest_ShutdownOrderPriorityAnnotation_Rule7a_RejectsWithDeferralMessage(t *testing.T) {
	err := loadFixture(t, "auto-shutdown-invalid-rule7a.yml")
	require.Error(t, err)
	require.Contains(t, err.Error(), "reserved for 3.24.7")
}

// TestParseAutoShutdownManifest_RecoveryModeScheduled_Rule8_RejectsWithDeferralMessage
// verifies rule 8: scheduled deferral.
func TestParseAutoShutdownManifest_RecoveryModeScheduled_Rule8_RejectsWithDeferralMessage(t *testing.T) {
	err := loadFixture(t, "auto-shutdown-invalid-rule8.yml")
	require.Error(t, err)
	require.Contains(t, err.Error(), "reserved for 3.25.0")
}

// TestParseAutoShutdownManifest_RecoveryModeInvalid_Rule9_RejectsHardFail
// verifies rule 9: invalid enum rejects.
func TestParseAutoShutdownManifest_RecoveryModeInvalid_Rule9_RejectsHardFail(t *testing.T) {
	err := loadFixture(t, "auto-shutdown-invalid-rule9.yml")
	require.Error(t, err)
	require.Contains(t, err.Error(), "must be one of")
}

// TestParseAutoShutdownManifest_AllServicesInNeverAutoShutdown_Rule10_RejectsHardFail
// verifies rule 10: all services exempt rejects.
func TestParseAutoShutdownManifest_AllServicesInNeverAutoShutdown_Rule10_RejectsHardFail(t *testing.T) {
	err := loadFixture(t, "auto-shutdown-invalid-rule10.yml")
	require.Error(t, err)
	require.Contains(t, err.Error(), "leaves no services eligible")
}

// TestParseAutoShutdownManifest_TimerOnlyApp_Rule10a_RejectsHardFail
// verifies rule 10a: no services block (timer-only) rejects.
func TestParseAutoShutdownManifest_TimerOnlyApp_Rule10a_RejectsHardFail(t *testing.T) {
	err := loadFixture(t, "auto-shutdown-invalid-rule10a.yml")
	require.Error(t, err)
	require.Contains(t, err.Error(), "requires at least one service")
}

// TestParseAutoShutdownManifest_NeverAutoShutdownContainsUnknownService_Rule3_WarnOnly
// verifies rule 3: unknown service in neverAutoShutdown does not fail
// validation (warning only). Stderr capture is best-effort; the
// load+validate must succeed.
func TestParseAutoShutdownManifest_NeverAutoShutdownContainsUnknownService_Rule3_WarnOnly(t *testing.T) {
	yamlBody := `services:
  api:
    image: example/api:latest
  worker:
    image: example/worker:latest

budget:
  monthlyCapUsd: 100
  atCapAction: auto-shutdown
  atCapWebhookUrl: https://hooks.example.com/budget
  neverAutoShutdown:
    - api
    - foo-not-a-service
`
	m, err := manifest.Load([]byte(yamlBody), map[string]string{})
	require.NoError(t, err)
	require.NoError(t, m.Validate())
}

// TestParseAutoShutdownManifest_NoBudgetBlock_Succeeds verifies that
// manifests without any budget block continue to work — the new
// validation pass must NOT regress backward compatibility.
func TestParseAutoShutdownManifest_NoBudgetBlock_Succeeds(t *testing.T) {
	yamlBody := `services:
  api:
    image: example/api:latest
`
	m, err := manifest.Load([]byte(yamlBody), map[string]string{})
	require.NoError(t, err)
	require.NoError(t, m.Validate())
}
