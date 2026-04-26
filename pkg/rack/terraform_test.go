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
