package rack

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEffectivePrometheusUrlSimplification_HashChange pins the post-redesign
// shape of effective_prometheus_url in terraform/system/aws/main.tf. The locals
// block must trivially alias var.prometheus_url; the rc8-shape 4-way priority
// ternary chain (var.monitoring_metrics_provisioned, var.gpu_observability_enable,
// hardcoded service URLs) must NOT appear. Catches regressions where someone
// reintroduces the auto-resolution chain.
//
// Mechanism mirrors TestReconcileVarsWithModule_PrometheusUrlInModuleVarsTf_AcceptedAsValid
// at terraform_test.go:302 — fixture-free os.ReadFile + assert.Contains/NotContains
// against the actual on-disk TF source.
func TestEffectivePrometheusUrlSimplification_HashChange(t *testing.T) {
	repoRoot, err := repoRootFromTestFile()
	require.NoError(t, err)
	mainPath := filepath.Join(repoRoot, "terraform", "system", "aws", "main.tf")

	data, err := os.ReadFile(mainPath)
	require.NoError(t, err, "must be able to read terraform/system/aws/main.tf")
	src := string(data)

	assert.Contains(t, src, `effective_prometheus_url = var.prometheus_url`,
		"locals must trivially alias var.prometheus_url post-redesign")
	assert.NotContains(t, src, `var.monitoring_metrics_provisioned ?`,
		"rc8-shape monitoring_metrics_provisioned ternary must not appear")
	assert.NotContains(t, src, `var.gpu_observability_enable ?`,
		"rc8-shape gpu_observability_enable ternary must not appear")
	assert.NotContains(t, src, `convox-kube-prometheus-sta-prometheus.convox-monitoring.svc.cluster.local:9090`,
		"hardcoded paid-path service URL must not appear in the local")
	assert.NotContains(t, src, `prometheus-gpu-metrics-server.kube-system.svc.cluster.local:80`,
		"hardcoded free-path service URL must not appear in the local")
}

// TestEffectivePrometheusUrlSimplification_HashChange_BoundedRestart pins the
// rolling-restart contract for the api-pod when var.prometheus_url changes.
// The kubernetes_deployment.api at terraform/api/k8s/main.tf has a
// convox.com/secret-checksum-prometheus-url annotation that hashes
// var.effective_prometheus_url; a change forces a pod-spec hash change, which
// triggers rolling restart. The rolling-update strategy with max_unavailable=0
// preserves continuous availability during the restart.
//
// progressDeadlineSeconds is NOT asserted: K8s default is 600s (10 min) and
// the field is NOT declared in the TF source — implicit default applies. The
// "≤30-90s typical" user-experience expectation is NOT
// asserted here (it's a user expectation, not a TF-source guarantee).
func TestEffectivePrometheusUrlSimplification_HashChange_BoundedRestart(t *testing.T) {
	repoRoot, err := repoRootFromTestFile()
	require.NoError(t, err)
	mainPath := filepath.Join(repoRoot, "terraform", "api", "k8s", "main.tf")

	data, err := os.ReadFile(mainPath)
	require.NoError(t, err, "must be able to read terraform/api/k8s/main.tf")
	src := string(data)

	assert.Contains(t, src, `convox.com/secret-checksum-prometheus-url`,
		"annotation key must be present so pod-spec hash changes when prometheus_url changes")
	assert.Contains(t, src, `sha256(var.effective_prometheus_url)`,
		"annotation value must wrap the variable in sha256() so the hash changes deterministically; matches the secret-checksum convention used for webhook_signing_key, docker_hub_password, api_password annotations")
	assert.Contains(t, src, `max_unavailable = 0`,
		"continuous-availability bound: max_unavailable must be 0 during rolling restart")
	assert.Contains(t, src, `type = "RollingUpdate"`,
		"explicit rolling-update strategy declaration required")
}
