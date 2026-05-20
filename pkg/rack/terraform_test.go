package rack

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/convox/stdcli"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// reconcileFixture builds a hermetic rack settings directory containing the
// minimum scaffolding `reconcileVarsWithModule` reads: vars.json,
// .terraform/modules/modules.json with one "system" entry, and a synthesized
// downloaded-module variables.tf at the manifest path. moduleVars defines the
// set of declared variables in the simulated downloaded module — these are
// what the target version of the system/aws module would declare. vars is
// what's currently persisted in vars.json before reconcile runs.
type reconcileFixture struct {
	rackName   string
	rackDir    string
	moduleVars []string
	vars       map[string]string
}

// setup writes the fixture state. The caller is responsible for the lifetime
// of settingsRoot (typically t.TempDir()).
func (f *reconcileFixture) setup(t *testing.T, settingsRoot string) {
	t.Helper()
	f.rackDir = filepath.Join(settingsRoot, "racks", f.rackName)
	require.NoError(t, os.MkdirAll(f.rackDir, 0700))

	// main.tf — content doesn't matter for moduleVarNames, but t.update() will
	// rewrite it via terraformWriteTemplate. We pre-create it so varsFile()'s
	// dir-stat passes and so update() can call os.Create on a real path.
	require.NoError(t, os.WriteFile(filepath.Join(f.rackDir, "main.tf"), []byte{}, 0600))

	// vars.json — what's currently persisted on disk.
	data, err := json.MarshalIndent(f.vars, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(f.rackDir, "vars.json"), data, 0600))

	// Synthesize a downloaded module containing variable declarations the
	// target version would expose. moduleVarNames() reads
	// .terraform/modules/modules.json to find the system module's directory,
	// then globs *.tf and regex-extracts variable blocks.
	moduleDirRel := ".terraform/modules/system"
	moduleDirAbs := filepath.Join(f.rackDir, moduleDirRel)
	require.NoError(t, os.MkdirAll(moduleDirAbs, 0700))

	var sb strings.Builder
	for _, v := range f.moduleVars {
		fmt.Fprintf(&sb, "variable %q { type = string }\n\n", v)
	}
	require.NoError(t, os.WriteFile(filepath.Join(moduleDirAbs, "variables.tf"), []byte(sb.String()), 0600))

	manifest := struct {
		Modules []struct {
			Key string `json:"Key"`
			Dir string `json:"Dir"`
		} `json:"Modules"`
	}{
		Modules: []struct {
			Key string `json:"Key"`
			Dir string `json:"Dir"`
		}{
			{Key: "system", Dir: moduleDirRel},
		},
	}
	manifestData, err := json.MarshalIndent(manifest, "", "  ")
	require.NoError(t, err)
	manifestPath := filepath.Join(f.rackDir, ".terraform", "modules", "modules.json")
	require.NoError(t, os.MkdirAll(filepath.Dir(manifestPath), 0700))
	require.NoError(t, os.WriteFile(manifestPath, manifestData, 0600))
}

// readVars loads vars.json from the fixture rack directory.
func (f *reconcileFixture) readVars(t *testing.T) map[string]string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(f.rackDir, "vars.json"))
	require.NoError(t, err)
	var got map[string]string
	require.NoError(t, json.Unmarshal(data, &got))
	return got
}

// withTerraform constructs a Terraform value pointing at the fixture's
// settings directory. It does this by registering a no-op command on a fresh
// stdcli.Engine and invoking the engine — the registered handler captures
// the *stdcli.Context that's needed by Terraform's settingsDirectory()
// chain. The returned string holds whatever the handler emits to os.Stderr
// (reconcileVarsWithModule writes its NOTICE there directly, bypassing the
// stdcli writer). os.Stderr is replaced with a pipe for the duration of the
// engine invocation and restored before this function returns, so tests
// must run any stderr assertions on the returned value AFTER withTerraform
// returns; reading the value inside the handler is too early because the
// pipe writer is still open at that point.
func withTerraform(t *testing.T, settingsRoot, rackName string, fn func(t *testing.T, tf Terraform)) string {
	t.Helper()

	e := stdcli.New("convox", "test")
	e.Settings = settingsRoot

	origStderr := os.Stderr
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stderr = w

	var captured string
	done := make(chan struct{})
	go func() {
		data, _ := io.ReadAll(r)
		captured = string(data)
		close(done)
	}()

	var ranHandler bool
	e.Command("rt", "reconcile-test handler", func(c *stdcli.Context) error {
		tf := Terraform{
			ctx:      c,
			name:     rackName,
			provider: "aws",
		}
		fn(t, tf)
		ranHandler = true
		return nil
	}, stdcli.CommandOptions{})

	code := e.Execute([]string{"rt"})
	require.NoError(t, w.Close(), "close stderr pipe writer")
	<-done
	os.Stderr = origStderr

	require.True(t, ranHandler, "test handler did not run")
	require.Equal(t, 0, code, "engine returned non-zero")

	return captured
}

// TestReconcileVarsWithModule_PrometheusUrlAcceptedByCurrentVersion exercises
// the happy path: the downloaded module declares prometheus_url, so reconcile
// must preserve it (no removal).
func TestReconcileVarsWithModule_PrometheusUrlAcceptedByCurrentVersion(t *testing.T) {
	settings := t.TempDir()
	f := reconcileFixture{
		rackName:   "test-rack",
		moduleVars: []string{"name", "release", "prometheus_url"},
		vars: map[string]string{
			"name":           "test-rack",
			"release":        "3.24.6",
			"prometheus_url": "https://prom.example.com",
		},
	}
	f.setup(t, settings)

	capturedStderr := withTerraform(t, settings, f.rackName, func(t *testing.T, tf Terraform) {
		require.NoError(t, tf.reconcileVarsWithModule("3.24.6"))
	})

	got := f.readVars(t)
	assert.Equal(t, "https://prom.example.com", got["prometheus_url"], "prometheus_url should be preserved when accepted by module")
	assert.Equal(t, "test-rack", got["name"])
	assert.Equal(t, "3.24.6", got["release"])
	assert.NotContains(t, capturedStderr, "removing parameters", "no NOTICE expected when nothing is removed")
}

// TestReconcileVarsWithModule_PrometheusUrlEmptyDefault_StaysClean exercises
// the empty-default scenario: vars.json carries prometheus_url="" and the
// module declares the variable. The reconciler MUST NOT flag it as
// "unrecognized" (writeVars's empty-stripping is a separate concern; the
// invariant under test is that reconcile does not emit a removal NOTICE for
// a key that is, in fact, declared by the target module).
func TestReconcileVarsWithModule_PrometheusUrlEmptyDefault_StaysClean(t *testing.T) {
	settings := t.TempDir()
	f := reconcileFixture{
		rackName:   "test-rack",
		moduleVars: []string{"name", "release", "prometheus_url"},
		vars: map[string]string{
			"name":           "test-rack",
			"release":        "3.24.6",
			"prometheus_url": "",
		},
	}
	f.setup(t, settings)

	capturedStderr := withTerraform(t, settings, f.rackName, func(t *testing.T, tf Terraform) {
		require.NoError(t, tf.reconcileVarsWithModule("3.24.6"))
	})

	assert.NotContains(t, capturedStderr, "removing parameters", "reconcile must not flag prometheus_url=\"\" as unrecognized when the module declares it")
}

// TestReconcileVarsWithModule_PrometheusUrlRemovedOnDowngrade is the core
// fingertrap exercise: the downgraded module does NOT declare prometheus_url
// but vars.json carries a non-empty value. Reconcile must (a) remove the key
// from vars.json, (b) emit a NOTICE on stderr, (c) leave name/release alone.
func TestReconcileVarsWithModule_PrometheusUrlRemovedOnDowngrade(t *testing.T) {
	settings := t.TempDir()
	f := reconcileFixture{
		rackName:   "test-rack",
		moduleVars: []string{"name", "release"}, // 3.24.5-like: no prometheus_url
		vars: map[string]string{
			"name":           "test-rack",
			"release":        "3.24.5",
			"prometheus_url": "https://prom.example.com",
		},
	}
	f.setup(t, settings)

	capturedStderr := withTerraform(t, settings, f.rackName, func(t *testing.T, tf Terraform) {
		require.NoError(t, tf.reconcileVarsWithModule("3.24.5"))
	})

	got := f.readVars(t)
	_, has := got["prometheus_url"]
	assert.False(t, has, "prometheus_url should be removed on downgrade to a module that does not declare it")
	assert.Equal(t, "test-rack", got["name"], "name must be preserved")
	assert.Equal(t, "3.24.5", got["release"], "release must be preserved")
	assert.Contains(t, capturedStderr, "removing parameters not supported by version 3.24.5", "NOTICE must be emitted")
	assert.Contains(t, capturedStderr, "prometheus_url", "NOTICE must name the removed key")
}

// TestReconcileVarsWithModule_PrometheusUrlRemovedFromMainTf verifies the
// main.tf rewrite path: when prometheus_url is removed from vars.json, the
// reconciler also rewrites main.tf via terraformWriteTemplate so the
// rendered module call no longer references the removed variable.
func TestReconcileVarsWithModule_PrometheusUrlRemovedFromMainTf(t *testing.T) {
	settings := t.TempDir()
	f := reconcileFixture{
		rackName:   "test-rack",
		moduleVars: []string{"name", "release"},
		vars: map[string]string{
			"name":           "test-rack",
			"release":        "3.24.5",
			"prometheus_url": "https://prom.example.com",
		},
	}
	f.setup(t, settings)

	withTerraform(t, settings, f.rackName, func(t *testing.T, tf Terraform) {
		require.NoError(t, tf.reconcileVarsWithModule("3.24.5"))
	})

	mainTfData, err := os.ReadFile(filepath.Join(f.rackDir, "main.tf"))
	require.NoError(t, err)
	assert.NotContains(t, string(mainTfData), "prometheus_url", "main.tf must not reference removed variable")
	assert.Contains(t, string(mainTfData), `name = "test-rack"`, "main.tf must still reference name")
	assert.Contains(t, string(mainTfData), `release = "3.24.5"`, "main.tf must still reference release")
}

// TestReconcileVarsWithModule_OtherVarsNotAffectedByPrometheusUrlReconcile
// regression-guards isolation: removing prometheus_url must not touch other
// vars that ARE declared by the downgraded module.
func TestReconcileVarsWithModule_OtherVarsNotAffectedByPrometheusUrlReconcile(t *testing.T) {
	settings := t.TempDir()
	f := reconcileFixture{
		rackName: "test-rack",
		moduleVars: []string{
			"name",
			"release",
			"karpenter_enabled",
			"cost_tracking_enable",
			"node_type",
		},
		vars: map[string]string{
			"name":                 "test-rack",
			"release":              "3.24.5",
			"karpenter_enabled":    "true",
			"cost_tracking_enable": "false",
			"node_type":            "t3.large",
			"prometheus_url":       "https://prom.example.com",
		},
	}
	f.setup(t, settings)

	withTerraform(t, settings, f.rackName, func(t *testing.T, tf Terraform) {
		require.NoError(t, tf.reconcileVarsWithModule("3.24.5"))
	})

	got := f.readVars(t)
	_, has := got["prometheus_url"]
	assert.False(t, has, "prometheus_url alone should be removed")
	assert.Equal(t, "true", got["karpenter_enabled"], "karpenter_enabled preserved")
	assert.Equal(t, "t3.large", got["node_type"], "node_type preserved")
	assert.Equal(t, "test-rack", got["name"])
	assert.Equal(t, "3.24.5", got["release"])
	// cost_tracking_enable is "false" — writeVars() strips empty values
	// but "false" is a non-empty string, so it is preserved.
	assert.Equal(t, "false", got["cost_tracking_enable"], "cost_tracking_enable preserved")
}

// TestReconcileVarsWithModule_PrometheusUrlInModuleVarsTf_AcceptedAsValid
// pins the convention: the actual on-disk system/aws/variables.tf must
// declare prometheus_url so that reconcileVarsWithModule will accept it
// when the target version is 3.24.6+. This is a fixture-free guard against
// accidental removal of the variable declaration in future refactors.
func TestReconcileVarsWithModule_PrometheusUrlInModuleVarsTf_AcceptedAsValid(t *testing.T) {
	repoRoot, err := repoRootFromTestFile()
	require.NoError(t, err)
	varsPath := filepath.Join(repoRoot, "terraform", "system", "aws", "variables.tf")

	data, err := os.ReadFile(varsPath)
	require.NoError(t, err, "must be able to read terraform/system/aws/variables.tf")
	assert.Contains(t, string(data), `variable "prometheus_url"`, "system/aws/variables.tf must declare prometheus_url")
	assert.NotContains(t, string(data), `variable "prometheus-url"`, "kebab-cased TF variable name must NOT appear (snake_case convention)")
}

// TestReconcileVarsWithModule_StripsGPUObservability_OnDowngrade is the
// fingertrap exercise for the Plan-5 GPU observability variables. The
// downgraded module declares neither gpu_observability_enable nor
// gpu_observability_chart_version; vars.json carries non-empty values for
// both. Reconcile must (a) remove BOTH keys from vars.json, (b) emit a
// NOTICE on stderr that names BOTH keys, (c) leave name/release untouched.
func TestReconcileVarsWithModule_StripsGPUObservability_OnDowngrade(t *testing.T) {
	settings := t.TempDir()
	f := reconcileFixture{
		rackName:   "test-rack",
		moduleVars: []string{"name", "release"}, // 3.24.5-like: no gpu_observability_*
		vars: map[string]string{
			"name":                            "test-rack",
			"release":                         "3.24.5",
			"gpu_observability_enable":        "true",
			"gpu_observability_chart_version": "4.8.1",
		},
	}
	f.setup(t, settings)

	capturedStderr := withTerraform(t, settings, f.rackName, func(t *testing.T, tf Terraform) {
		require.NoError(t, tf.reconcileVarsWithModule("3.24.5"))
	})

	got := f.readVars(t)
	_, hasEnable := got["gpu_observability_enable"]
	_, hasVersion := got["gpu_observability_chart_version"]
	assert.False(t, hasEnable, "gpu_observability_enable should be removed on downgrade")
	assert.False(t, hasVersion, "gpu_observability_chart_version should be removed on downgrade")
	assert.Equal(t, "test-rack", got["name"], "name must be preserved")
	assert.Equal(t, "3.24.5", got["release"], "release must be preserved")
	assert.Contains(t, capturedStderr, "removing parameters not supported by version 3.24.5", "NOTICE must be emitted")
	assert.Contains(t, capturedStderr, "gpu_observability_enable", "NOTICE must name gpu_observability_enable")
	assert.Contains(t, capturedStderr, "gpu_observability_chart_version", "NOTICE must name gpu_observability_chart_version")
}

// TestReconcileVarsWithModule_GPUObservabilityAcceptedByCurrentVersion
// exercises the happy path: when the target module declares both
// gpu_observability_* variables, reconcile preserves them.
func TestReconcileVarsWithModule_GPUObservabilityAcceptedByCurrentVersion(t *testing.T) {
	settings := t.TempDir()
	f := reconcileFixture{
		rackName:   "test-rack",
		moduleVars: []string{"name", "release", "gpu_observability_enable", "gpu_observability_chart_version"},
		vars: map[string]string{
			"name":                            "test-rack",
			"release":                         "3.24.6",
			"gpu_observability_enable":        "true",
			"gpu_observability_chart_version": "4.8.1",
		},
	}
	f.setup(t, settings)

	capturedStderr := withTerraform(t, settings, f.rackName, func(t *testing.T, tf Terraform) {
		require.NoError(t, tf.reconcileVarsWithModule("3.24.6"))
	})

	got := f.readVars(t)
	assert.Equal(t, "true", got["gpu_observability_enable"], "gpu_observability_enable preserved when accepted")
	assert.Equal(t, "4.8.1", got["gpu_observability_chart_version"], "gpu_observability_chart_version preserved when accepted")
	assert.NotContains(t, capturedStderr, "removing parameters", "no NOTICE expected when nothing is removed")
}

// TestReconcileVarsWithModule_GPUObservabilityInModuleVarsTf_AcceptedAsValid
// pins the convention: the actual on-disk system/aws/variables.tf must
// declare both gpu_observability_* variables so that reconcileVarsWithModule
// will accept them when the target version is 3.24.6+. This is a
// fixture-free guard against accidental removal of the variable declarations
// in future refactors (mirrors the prometheus_url variant above).
func TestReconcileVarsWithModule_GPUObservabilityInModuleVarsTf_AcceptedAsValid(t *testing.T) {
	repoRoot, err := repoRootFromTestFile()
	require.NoError(t, err)
	varsPath := filepath.Join(repoRoot, "terraform", "system", "aws", "variables.tf")

	data, err := os.ReadFile(varsPath)
	require.NoError(t, err, "must be able to read terraform/system/aws/variables.tf")
	assert.Contains(t, string(data), `variable "gpu_observability_enable"`, "system/aws/variables.tf must declare gpu_observability_enable")
	assert.Contains(t, string(data), `variable "gpu_observability_chart_version"`, "system/aws/variables.tf must declare gpu_observability_chart_version")
}

// TestReconcileVarsWithModule_StripsWebhookSigningKey_OnDowngrade is the
// fingertrap exercise for webhook_signing_key — the bundle's most security-
// sensitive new variable (a user-supplied HMAC secret) and the only
// non-AWS-only new variable (declared in azure/gcp/do/local/metal system
// modules too). The downgraded module does NOT declare webhook_signing_key
// but vars.json carries a non-empty value. Reconcile must (a) remove the key
// from vars.json, (b) emit a NOTICE on stderr that names the removed key,
// (c) leave name/release alone. Mirrors the prometheus_url variant above.
func TestReconcileVarsWithModule_StripsWebhookSigningKey_OnDowngrade(t *testing.T) {
	settings := t.TempDir()
	f := reconcileFixture{
		rackName:   "test-rack",
		moduleVars: []string{"name", "release"}, // 3.24.5-like: no webhook_signing_key
		vars: map[string]string{
			"name":                "test-rack",
			"release":             "3.24.5",
			"webhook_signing_key": "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		},
	}
	f.setup(t, settings)

	capturedStderr := withTerraform(t, settings, f.rackName, func(t *testing.T, tf Terraform) {
		require.NoError(t, tf.reconcileVarsWithModule("3.24.5"))
	})

	got := f.readVars(t)
	_, has := got["webhook_signing_key"]
	assert.False(t, has, "webhook_signing_key should be removed on downgrade to a module that does not declare it")
	assert.Equal(t, "test-rack", got["name"], "name must be preserved")
	assert.Equal(t, "3.24.5", got["release"], "release must be preserved")
	assert.Contains(t, capturedStderr, "removing parameters not supported by version 3.24.5", "NOTICE must be emitted")
	assert.Contains(t, capturedStderr, "webhook_signing_key", "NOTICE must name the removed key")
}

// TestReconcileVarsWithModule_WebhookSigningKeyAcceptedByCurrentVersion
// exercises the happy path: the downloaded module declares webhook_signing_key,
// so reconcile must preserve it (no removal).
func TestReconcileVarsWithModule_WebhookSigningKeyAcceptedByCurrentVersion(t *testing.T) {
	settings := t.TempDir()
	f := reconcileFixture{
		rackName:   "test-rack",
		moduleVars: []string{"name", "release", "webhook_signing_key"},
		vars: map[string]string{
			"name":                "test-rack",
			"release":             "3.24.6",
			"webhook_signing_key": "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		},
	}
	f.setup(t, settings)

	capturedStderr := withTerraform(t, settings, f.rackName, func(t *testing.T, tf Terraform) {
		require.NoError(t, tf.reconcileVarsWithModule("3.24.6"))
	})

	got := f.readVars(t)
	assert.Equal(t, "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef", got["webhook_signing_key"], "webhook_signing_key should be preserved when accepted by module")
	assert.Equal(t, "test-rack", got["name"])
	assert.Equal(t, "3.24.6", got["release"])
	assert.NotContains(t, capturedStderr, "removing parameters", "no NOTICE expected when nothing is removed")
}

// TestReconcileVarsWithModule_WebhookSigningKeyInModuleVarsTf_AcceptedAsValid
// pins the convention: the actual on-disk system/aws/variables.tf must
// declare webhook_signing_key so that reconcileVarsWithModule will accept it
// when the target version is 3.24.6+. Fixture-free guard against accidental
// removal in future refactors (mirrors the prometheus_url and gpu_observability
// variants above).
func TestReconcileVarsWithModule_WebhookSigningKeyInModuleVarsTf_AcceptedAsValid(t *testing.T) {
	repoRoot, err := repoRootFromTestFile()
	require.NoError(t, err)
	varsPath := filepath.Join(repoRoot, "terraform", "system", "aws", "variables.tf")

	data, err := os.ReadFile(varsPath)
	require.NoError(t, err, "must be able to read terraform/system/aws/variables.tf")
	assert.Contains(t, string(data), `variable "webhook_signing_key"`, "system/aws/variables.tf must declare webhook_signing_key")
	assert.NotContains(t, string(data), `variable "webhook-signing-key"`, "kebab-cased TF variable name must NOT appear (snake_case convention)")
}

// repoRootFromTestFile walks up from the test's working directory to the
// repo root by looking for go.mod. Used to read real on-disk module sources
// to verify conventions.
func repoRootFromTestFile() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	dir := wd
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("go.mod not found from %s", wd)
		}
		dir = parent
	}
}

// TestReconcileVarsWithModule_StripsMonitoringMetricsProvisioned_OnDowngrade is
// the fingertrap exercise for the rc8→3.24.6-final transition. The downgraded
// (3.24.6-final) module no longer declares monitoring_metrics_provisioned;
// vars.json carries it from the rc8-era state. Reconcile must (a) remove the
// key from vars.json, (b) emit a NOTICE on stderr that names it, (c) leave
// name/release untouched.
func TestReconcileVarsWithModule_StripsMonitoringMetricsProvisioned_OnDowngrade(t *testing.T) {
	settings := t.TempDir()
	f := reconcileFixture{
		rackName:   "test-rack",
		moduleVars: []string{"name", "release"}, // 3.24.6-final-like: no monitoring_metrics_provisioned
		vars: map[string]string{
			"name":                           "test-rack",
			"release":                        "3.24.5",
			"monitoring_metrics_provisioned": "true",
		},
	}
	f.setup(t, settings)

	capturedStderr := withTerraform(t, settings, f.rackName, func(t *testing.T, tf Terraform) {
		require.NoError(t, tf.reconcileVarsWithModule("3.24.5"))
	})

	got := f.readVars(t)
	_, has := got["monitoring_metrics_provisioned"]
	assert.False(t, has, "monitoring_metrics_provisioned should be removed on downgrade")
	assert.Equal(t, "test-rack", got["name"], "name must be preserved")
	assert.Equal(t, "3.24.5", got["release"], "release must be preserved")
	assert.Contains(t, capturedStderr, "removing parameters not supported by version 3.24.5", "NOTICE must be emitted")
	assert.Contains(t, capturedStderr, "monitoring_metrics_provisioned", "NOTICE must name monitoring_metrics_provisioned")

	// main.tf rewrite assertion (mirrors TestReconcileVarsWithModule_PrometheusUrlRemovedFromMainTf
	// at lines 232-254): the reconciler's t.update(release, vars) rewrites both
	// vars.json AND main.tf; if the main.tf rewrite regresses for the
	// cleaned-vars path, terraform apply would fail with "argument not expected"
	// — exactly the fingertrap class this test exists to catch.
	mainTfData, err := os.ReadFile(filepath.Join(f.rackDir, "main.tf"))
	require.NoError(t, err)
	assert.NotContains(t, string(mainTfData), "monitoring_metrics_provisioned", "main.tf must not reference removed variable")
	assert.Contains(t, string(mainTfData), `name = "test-rack"`, "main.tf must still reference name")
	assert.Contains(t, string(mainTfData), `release = "3.24.5"`, "main.tf must still reference release")
}

// TestReconcileVarsWithModule_StripsOrphanedPromVars_OnDowngrade is a
// fixture-only future-proofing exercise. The 3.24.5 system/aws/variables.tf
// STILL declares prometheus_gpu_metrics_chart_version + retention, and 3.24.6
// KEEPS them for SystemRaw consumption — so neither version
// actually triggers stripping in real downgrades. This test uses synthetic
// moduleVars excluding both prom-params to exercise the reconciler in a
// hypothetical future-removal path. Catches regressions in future minor-bump
// downgrades that drop the prom-vars from variables.tf.
func TestReconcileVarsWithModule_StripsOrphanedPromVars_OnDowngrade(t *testing.T) {
	settings := t.TempDir()
	f := reconcileFixture{
		rackName:   "test-rack",
		moduleVars: []string{"name", "release"}, // synthetic; excludes prom-vars
		vars: map[string]string{
			"name":                                 "test-rack",
			"release":                              "3.24.5",
			"prometheus_gpu_metrics_chart_version": "27.9.0",
			"prometheus_gpu_metrics_retention":     "24h",
		},
	}
	f.setup(t, settings)

	capturedStderr := withTerraform(t, settings, f.rackName, func(t *testing.T, tf Terraform) {
		require.NoError(t, tf.reconcileVarsWithModule("3.24.5"))
	})

	got := f.readVars(t)
	_, hasVer := got["prometheus_gpu_metrics_chart_version"]
	_, hasRet := got["prometheus_gpu_metrics_retention"]
	assert.False(t, hasVer, "prometheus_gpu_metrics_chart_version should be removed when module no longer declares it")
	assert.False(t, hasRet, "prometheus_gpu_metrics_retention should be removed when module no longer declares it")
	assert.Equal(t, "test-rack", got["name"], "name must be preserved")
	assert.Contains(t, capturedStderr, "prometheus_gpu_metrics_chart_version", "NOTICE must name prometheus_gpu_metrics_chart_version")
	assert.Contains(t, capturedStderr, "prometheus_gpu_metrics_retention", "NOTICE must name prometheus_gpu_metrics_retention")

	mainTfData, err := os.ReadFile(filepath.Join(f.rackDir, "main.tf"))
	require.NoError(t, err)
	assert.NotContains(t, string(mainTfData), "prometheus_gpu_metrics_chart_version", "main.tf must not reference removed variable")
	assert.NotContains(t, string(mainTfData), "prometheus_gpu_metrics_retention", "main.tf must not reference removed variable")
	assert.Contains(t, string(mainTfData), `name = "test-rack"`, "main.tf must still reference name")
}

// TestReconcileVarsWithModule_StripsMonitoringMetricsProvisioned_OnRc8ToFinalUpgrade
// pins reconciler behavior at the unit-test layer. vars.json
// contains monitoring_metrics_provisioned=true (rc8 paid-monitoring state); module
// variables.tf does NOT declare it (3.24.6-final). Reconciler strips the var and
// emits NOTICE — same mechanism as the downgrade path. Test catches future
// regressions in reconciler upgrade-path orphan stripping.
func TestReconcileVarsWithModule_StripsMonitoringMetricsProvisioned_OnRc8ToFinalUpgrade(t *testing.T) {
	settings := t.TempDir()
	f := reconcileFixture{
		rackName:   "test-rack",
		moduleVars: []string{"name", "release", "gpu_observability_enable"}, // 3.24.6-final declares these
		vars: map[string]string{
			"name":                           "test-rack",
			"release":                        "3.24.6",
			"monitoring_metrics_provisioned": "true",
			"gpu_observability_enable":       "true",
		},
	}
	f.setup(t, settings)

	capturedStderr := withTerraform(t, settings, f.rackName, func(t *testing.T, tf Terraform) {
		require.NoError(t, tf.reconcileVarsWithModule("3.24.6"))
	})

	got := f.readVars(t)
	_, hasMMP := got["monitoring_metrics_provisioned"]
	assert.False(t, hasMMP, "monitoring_metrics_provisioned should be stripped on rc8→final upgrade")
	assert.Equal(t, "true", got["gpu_observability_enable"], "gpu_observability_enable must be preserved")
	assert.Equal(t, "test-rack", got["name"], "name must be preserved")
	assert.Equal(t, "3.24.6", got["release"], "release must reflect the 3.24.6 final upgrade target")
	assert.Contains(t, capturedStderr, "removing parameters not supported by version 3.24.6", "NOTICE must be emitted")
	assert.Contains(t, capturedStderr, "monitoring_metrics_provisioned", "NOTICE must name monitoring_metrics_provisioned")

	mainTfData, err := os.ReadFile(filepath.Join(f.rackDir, "main.tf"))
	require.NoError(t, err)
	assert.NotContains(t, string(mainTfData), "monitoring_metrics_provisioned", "main.tf must not reference removed variable")
	assert.Contains(t, string(mainTfData), `gpu_observability_enable = "true"`, "main.tf must still reference gpu_observability_enable")
	assert.Contains(t, string(mainTfData), `name = "test-rack"`, "main.tf must still reference name")
}

// TestReconcile_RemovesDcgmScrapeInterval is the fingertrap exercise for the
// dcgm_scrape_interval variable introduced in 3.24.6. The downgraded module
// does NOT declare the variable but vars.json carries a non-empty value;
// reconcile must (a) remove the key from vars.json, (b) emit a NOTICE on
// stderr that names the removed key, (c) leave name/release alone, (d) rewrite
// main.tf without referencing the removed variable.
func TestReconcile_RemovesDcgmScrapeInterval(t *testing.T) {
	settings := t.TempDir()
	f := reconcileFixture{
		rackName:   "test-rack",
		moduleVars: []string{"name", "release"}, // 3.24.5-like: no dcgm_scrape_interval
		vars: map[string]string{
			"name":                 "test-rack",
			"release":              "3.24.5",
			"dcgm_scrape_interval": "30s",
		},
	}
	f.setup(t, settings)

	capturedStderr := withTerraform(t, settings, f.rackName, func(t *testing.T, tf Terraform) {
		require.NoError(t, tf.reconcileVarsWithModule("3.24.5"))
	})

	got := f.readVars(t)
	_, has := got["dcgm_scrape_interval"]
	assert.False(t, has, "dcgm_scrape_interval should be removed on downgrade to a module that does not declare it")
	assert.Equal(t, "test-rack", got["name"], "name must be preserved")
	assert.Equal(t, "3.24.5", got["release"], "release must be preserved")
	assert.Contains(t, capturedStderr, "removing parameters not supported by version 3.24.5", "NOTICE must be emitted")
	assert.Contains(t, capturedStderr, "dcgm_scrape_interval", "NOTICE must name the removed key")

	mainTfData, err := os.ReadFile(filepath.Join(f.rackDir, "main.tf"))
	require.NoError(t, err)
	assert.NotContains(t, string(mainTfData), "dcgm_scrape_interval", "main.tf must not reference removed variable")
	assert.Contains(t, string(mainTfData), `name = "test-rack"`, "main.tf must still reference name")
}

// TestReconcile_RemovesGpuMetricsMaxPods is the fingertrap exercise for the
// gpu_metrics_max_pods variable introduced in 3.24.6. Pattern matches
// TestReconcile_RemovesDcgmScrapeInterval — the variable's classification is
// "process-config" (handler-side enforcement) so the reconciler removes it
// cleanly on downgrade and the handler falls back to its default 100.
func TestReconcile_RemovesGpuMetricsMaxPods(t *testing.T) {
	settings := t.TempDir()
	f := reconcileFixture{
		rackName:   "test-rack",
		moduleVars: []string{"name", "release"},
		vars: map[string]string{
			"name":                 "test-rack",
			"release":              "3.24.5",
			"gpu_metrics_max_pods": "200",
		},
	}
	f.setup(t, settings)

	capturedStderr := withTerraform(t, settings, f.rackName, func(t *testing.T, tf Terraform) {
		require.NoError(t, tf.reconcileVarsWithModule("3.24.5"))
	})

	got := f.readVars(t)
	_, has := got["gpu_metrics_max_pods"]
	assert.False(t, has, "gpu_metrics_max_pods should be removed on downgrade")
	assert.Equal(t, "test-rack", got["name"], "name must be preserved")
	assert.Equal(t, "3.24.5", got["release"], "release must be preserved")
	assert.Contains(t, capturedStderr, "removing parameters not supported by version 3.24.5", "NOTICE must be emitted")
	assert.Contains(t, capturedStderr, "gpu_metrics_max_pods", "NOTICE must name the removed key")

	mainTfData, err := os.ReadFile(filepath.Join(f.rackDir, "main.tf"))
	require.NoError(t, err)
	assert.NotContains(t, string(mainTfData), "gpu_metrics_max_pods", "main.tf must not reference removed variable")
}

// TestReconcile_RemovesGpuMetricsMaxConcurrent is the fingertrap exercise for
// the gpu_metrics_max_concurrent variable introduced in 3.24.6. Process-config
// classification — handler-side semaphore falls back to default 10.
func TestReconcile_RemovesGpuMetricsMaxConcurrent(t *testing.T) {
	settings := t.TempDir()
	f := reconcileFixture{
		rackName:   "test-rack",
		moduleVars: []string{"name", "release"},
		vars: map[string]string{
			"name":                       "test-rack",
			"release":                    "3.24.5",
			"gpu_metrics_max_concurrent": "20",
		},
	}
	f.setup(t, settings)

	capturedStderr := withTerraform(t, settings, f.rackName, func(t *testing.T, tf Terraform) {
		require.NoError(t, tf.reconcileVarsWithModule("3.24.5"))
	})

	got := f.readVars(t)
	_, has := got["gpu_metrics_max_concurrent"]
	assert.False(t, has, "gpu_metrics_max_concurrent should be removed on downgrade")
	assert.Equal(t, "test-rack", got["name"], "name must be preserved")
	assert.Equal(t, "3.24.5", got["release"], "release must be preserved")
	assert.Contains(t, capturedStderr, "removing parameters not supported by version 3.24.5", "NOTICE must be emitted")
	assert.Contains(t, capturedStderr, "gpu_metrics_max_concurrent", "NOTICE must name the removed key")

	mainTfData, err := os.ReadFile(filepath.Join(f.rackDir, "main.tf"))
	require.NoError(t, err)
	assert.NotContains(t, string(mainTfData), "gpu_metrics_max_concurrent", "main.tf must not reference removed variable")
}

// TestReconcile_RemovesReleaseWatcherGCInterval is the fingertrap exercise for
// the release_watcher_gc_interval variable introduced in 3.24.6. The variable
// is plumbed system→rack→api and surfaces as the RELEASE_WATCHER_GC_INTERVAL
// env var on the api Deployment. On downgrade the api Deployment env block
// loses the entry; the provider package falls back to its 5m default.
func TestReconcile_RemovesReleaseWatcherGCInterval(t *testing.T) {
	settings := t.TempDir()
	f := reconcileFixture{
		rackName:   "test-rack",
		moduleVars: []string{"name", "release"},
		vars: map[string]string{
			"name":                        "test-rack",
			"release":                     "3.24.5",
			"release_watcher_gc_interval": "10m",
		},
	}
	f.setup(t, settings)

	capturedStderr := withTerraform(t, settings, f.rackName, func(t *testing.T, tf Terraform) {
		require.NoError(t, tf.reconcileVarsWithModule("3.24.5"))
	})

	got := f.readVars(t)
	_, has := got["release_watcher_gc_interval"]
	assert.False(t, has, "release_watcher_gc_interval should be removed on downgrade")
	assert.Equal(t, "test-rack", got["name"], "name must be preserved")
	assert.Equal(t, "3.24.5", got["release"], "release must be preserved")
	assert.Contains(t, capturedStderr, "removing parameters not supported by version 3.24.5", "NOTICE must be emitted")
	assert.Contains(t, capturedStderr, "release_watcher_gc_interval", "NOTICE must name the removed key")

	mainTfData, err := os.ReadFile(filepath.Join(f.rackDir, "main.tf"))
	require.NoError(t, err)
	assert.NotContains(t, string(mainTfData), "release_watcher_gc_interval", "main.tf must not reference removed variable")
}

// TestReconcile_RemovesGrafanaDashboardVarRack is the fingertrap exercise for
// the grafana_dashboard_var_rack variable introduced in 3.24.6. Process-config:
// console reads the value via rack-params and substitutes into Grafana deep
// links; on downgrade the absent value falls back to the canonical default
// "rack" in console code.
func TestReconcile_RemovesGrafanaDashboardVarRack(t *testing.T) {
	settings := t.TempDir()
	f := reconcileFixture{
		rackName:   "test-rack",
		moduleVars: []string{"name", "release"},
		vars: map[string]string{
			"name":                       "test-rack",
			"release":                    "3.24.5",
			"grafana_dashboard_var_rack": "cluster_name",
		},
	}
	f.setup(t, settings)

	capturedStderr := withTerraform(t, settings, f.rackName, func(t *testing.T, tf Terraform) {
		require.NoError(t, tf.reconcileVarsWithModule("3.24.5"))
	})

	got := f.readVars(t)
	_, has := got["grafana_dashboard_var_rack"]
	assert.False(t, has, "grafana_dashboard_var_rack should be removed on downgrade")
	assert.Equal(t, "test-rack", got["name"], "name must be preserved")
	assert.Equal(t, "3.24.5", got["release"], "release must be preserved")
	assert.Contains(t, capturedStderr, "removing parameters not supported by version 3.24.5", "NOTICE must be emitted")
	assert.Contains(t, capturedStderr, "grafana_dashboard_var_rack", "NOTICE must name the removed key")

	mainTfData, err := os.ReadFile(filepath.Join(f.rackDir, "main.tf"))
	require.NoError(t, err)
	assert.NotContains(t, string(mainTfData), "grafana_dashboard_var_rack", "main.tf must not reference removed variable")
}

// TestReconcile_RemovesGrafanaDashboardVarNamespace mirrors the var_rack test
// for grafana_dashboard_var_namespace.
func TestReconcile_RemovesGrafanaDashboardVarNamespace(t *testing.T) {
	settings := t.TempDir()
	f := reconcileFixture{
		rackName:   "test-rack",
		moduleVars: []string{"name", "release"},
		vars: map[string]string{
			"name":                            "test-rack",
			"release":                         "3.24.5",
			"grafana_dashboard_var_namespace": "k8s_namespace",
		},
	}
	f.setup(t, settings)

	capturedStderr := withTerraform(t, settings, f.rackName, func(t *testing.T, tf Terraform) {
		require.NoError(t, tf.reconcileVarsWithModule("3.24.5"))
	})

	got := f.readVars(t)
	_, has := got["grafana_dashboard_var_namespace"]
	assert.False(t, has, "grafana_dashboard_var_namespace should be removed on downgrade")
	assert.Equal(t, "test-rack", got["name"], "name must be preserved")
	assert.Equal(t, "3.24.5", got["release"], "release must be preserved")
	assert.Contains(t, capturedStderr, "removing parameters not supported by version 3.24.5", "NOTICE must be emitted")
	assert.Contains(t, capturedStderr, "grafana_dashboard_var_namespace", "NOTICE must name the removed key")

	mainTfData, err := os.ReadFile(filepath.Join(f.rackDir, "main.tf"))
	require.NoError(t, err)
	assert.NotContains(t, string(mainTfData), "grafana_dashboard_var_namespace", "main.tf must not reference removed variable")
}

// TestReconcile_RemovesGrafanaDashboardVarService mirrors the var_rack test
// for grafana_dashboard_var_service.
func TestReconcile_RemovesGrafanaDashboardVarService(t *testing.T) {
	settings := t.TempDir()
	f := reconcileFixture{
		rackName:   "test-rack",
		moduleVars: []string{"name", "release"},
		vars: map[string]string{
			"name":                          "test-rack",
			"release":                       "3.24.5",
			"grafana_dashboard_var_service": "workload",
		},
	}
	f.setup(t, settings)

	capturedStderr := withTerraform(t, settings, f.rackName, func(t *testing.T, tf Terraform) {
		require.NoError(t, tf.reconcileVarsWithModule("3.24.5"))
	})

	got := f.readVars(t)
	_, has := got["grafana_dashboard_var_service"]
	assert.False(t, has, "grafana_dashboard_var_service should be removed on downgrade")
	assert.Equal(t, "test-rack", got["name"], "name must be preserved")
	assert.Equal(t, "3.24.5", got["release"], "release must be preserved")
	assert.Contains(t, capturedStderr, "removing parameters not supported by version 3.24.5", "NOTICE must be emitted")
	assert.Contains(t, capturedStderr, "grafana_dashboard_var_service", "NOTICE must name the removed key")

	mainTfData, err := os.ReadFile(filepath.Join(f.rackDir, "main.tf"))
	require.NoError(t, err)
	assert.NotContains(t, string(mainTfData), "grafana_dashboard_var_service", "main.tf must not reference removed variable")
}

// TestReconcile_RemovesGrafanaDashboardVarApp mirrors the var_rack test
// for grafana_dashboard_var_app.
func TestReconcile_RemovesGrafanaDashboardVarApp(t *testing.T) {
	settings := t.TempDir()
	f := reconcileFixture{
		rackName:   "test-rack",
		moduleVars: []string{"name", "release"},
		vars: map[string]string{
			"name":                      "test-rack",
			"release":                   "3.24.5",
			"grafana_dashboard_var_app": "application",
		},
	}
	f.setup(t, settings)

	capturedStderr := withTerraform(t, settings, f.rackName, func(t *testing.T, tf Terraform) {
		require.NoError(t, tf.reconcileVarsWithModule("3.24.5"))
	})

	got := f.readVars(t)
	_, has := got["grafana_dashboard_var_app"]
	assert.False(t, has, "grafana_dashboard_var_app should be removed on downgrade")
	assert.Equal(t, "test-rack", got["name"], "name must be preserved")
	assert.Equal(t, "3.24.5", got["release"], "release must be preserved")
	assert.Contains(t, capturedStderr, "removing parameters not supported by version 3.24.5", "NOTICE must be emitted")
	assert.Contains(t, capturedStderr, "grafana_dashboard_var_app", "NOTICE must name the removed key")

	mainTfData, err := os.ReadFile(filepath.Join(f.rackDir, "main.tf"))
	require.NoError(t, err)
	assert.NotContains(t, string(mainTfData), "grafana_dashboard_var_app", "main.tf must not reference removed variable")
}
