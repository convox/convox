package rack

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestStuckHelmSecrets(t *testing.T) {
	now := time.Date(2026, 7, 23, 12, 0, 0, 0, time.UTC)
	allow := convoxHelmReleases("myrack")

	stale := now.Add(-1 * time.Hour)
	fresh := now.Add(-2 * time.Minute)

	secret := func(name, release, ns, status string, created time.Time) helmReleaseSecret {
		return helmReleaseSecret{secretName: name, release: release, namespace: ns, status: status, version: "1", created: created}
	}

	found := []helmReleaseSecret{
		secret("sh.helm.release.v1.aws-lbc.v3", "aws-lbc", "kube-system", "pending-upgrade", stale),
		secret("sh.helm.release.v1.karpenter.v2", "karpenter", "kube-system", "pending-install", stale),
		secret("sh.helm.release.v1.contour.v5", "contour", "myrack-system", "pending-upgrade", stale),
		secret("sh.helm.release.v1.keda.v1", "keda", "keda", "deployed", stale),
		secret("sh.helm.release.v1.vpa.v4", "vpa", "vpa", "superseded", stale),
		secret("sh.helm.release.v1.aws-lbc.v2", "aws-lbc", "kube-system", "failed", stale),
		secret("sh.helm.release.v1.dcgm.v1", "dcgm-exporter", "kube-system", "pending-upgrade", fresh),
		secret("sh.helm.release.v1.keda.v9", "keda", "default", "pending-upgrade", stale),
		secret("sh.helm.release.v1.datadog.v1", "datadog", "monitoring", "pending-upgrade", stale),
		secret("sh.helm.release.v1.contour.v1", "contour", "kube-system", "pending-upgrade", stale),
	}

	got := stuckHelmSecrets(found, allow, now, helmStuckMinAge)

	var names []string
	for _, s := range got {
		names = append(names, s.secretName)
	}

	assert.ElementsMatch(t, []string{
		"sh.helm.release.v1.aws-lbc.v3",
		"sh.helm.release.v1.karpenter.v2",
		"sh.helm.release.v1.contour.v5",
	}, names)
}

func TestStuckHelmSecretsEmpty(t *testing.T) {
	now := time.Date(2026, 7, 23, 12, 0, 0, 0, time.UTC)
	assert.Empty(t, stuckHelmSecrets(nil, convoxHelmReleases("myrack"), now, helmStuckMinAge))
}

func TestEksClientPrivateHost(t *testing.T) {
	prefix := "/proxy/rid-1234/private/eks"

	var seen []*http.Request

	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.Clone(context.Background()))

		w.Header().Set("Content-Type", "application/json")

		switch r.Method {
		case "DELETE":
			fmt.Fprint(w, `{"kind":"Status","apiVersion":"v1","status":"Success"}`)
		default:
			fmt.Fprint(w, `{"kind":"SecretList","apiVersion":"v1","items":[]}`)
		}
	}))
	defer srv.Close()

	client, err := eksClient(context.Background(), "myrack", map[string]string{
		"private_eks_host": srv.URL + prefix + "/",
		"private_eks_user": "user-1",
		"private_eks_pass": "pass-1",
	})
	require.NoError(t, err)

	_, err = client.CoreV1().Secrets(metav1.NamespaceAll).List(context.Background(), metav1.ListOptions{LabelSelector: "owner=helm"})
	require.NoError(t, err)

	require.NoError(t, client.CoreV1().Secrets("kube-system").Delete(context.Background(), "sh.helm.release.v1.aws-lbc.v3", metav1.DeleteOptions{}))

	require.Len(t, seen, 2)

	list := seen[0]
	assert.Equal(t, "GET", list.Method)
	assert.Equal(t, prefix+"/api/v1/secrets", list.URL.Path)
	assert.Equal(t, "owner=helm", list.URL.Query().Get("labelSelector"))

	user, pass, ok := list.BasicAuth()
	assert.True(t, ok)
	assert.Equal(t, "user-1", user)
	assert.Equal(t, "pass-1", pass)

	del := seen[1]
	assert.Equal(t, "DELETE", del.Method)
	assert.Equal(t, prefix+"/api/v1/namespaces/kube-system/secrets/sh.helm.release.v1.aws-lbc.v3", del.URL.Path)
}

func TestEksClientPrivateHostWithoutUser(t *testing.T) {
	var user, pass string
	var ok bool

	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok = r.BasicAuth()
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"kind":"SecretList","apiVersion":"v1","items":[]}`)
	}))
	defer srv.Close()

	client, err := eksClient(context.Background(), "myrack", map[string]string{
		"private_eks_host": srv.URL + "/proxy/rid-1234/private/eks/",
		"private_eks_pass": "pass-1",
	})
	require.NoError(t, err)

	_, err = client.CoreV1().Secrets(metav1.NamespaceAll).List(context.Background(), metav1.ListOptions{})
	require.NoError(t, err)

	assert.True(t, ok)
	assert.NotEmpty(t, user)
	assert.Equal(t, "pass-1", pass)
}

func helmSecretJSON(name, release, namespace, status, version string, created time.Time) string {
	return fmt.Sprintf(`{"metadata":{"name":%q,"namespace":%q,"creationTimestamp":%q,"labels":{"owner":"helm","name":%q,"status":%q,"version":%q}},"type":%q}`,
		name, namespace, created.UTC().Format(time.RFC3339), release, status, version, helmReleaseType)
}

// writeRackVars lays down the minimum on-disk state t.vars() reads.
func writeRackVars(t *testing.T, settingsRoot, rack string, vars map[string]string) {
	t.Helper()

	dir := filepath.Join(settingsRoot, "racks", rack)
	require.NoError(t, os.MkdirAll(dir, 0700))

	data, err := json.Marshal(vars)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "vars.json"), data, 0600))
}

func TestReconcileStuckHelmReleasesPrivate(t *testing.T) {
	stale := time.Now().Add(-1 * time.Hour)
	fresh := time.Now().Add(-2 * time.Minute)

	var deleted []string

	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "DELETE" {
			deleted = append(deleted, r.URL.Path)

			if strings.Contains(r.URL.Path, "karpenter") {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"kind":"Status","apiVersion":"v1","status":"Success"}`)

			return
		}

		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"kind":"SecretList","apiVersion":"v1","items":[%s,%s,%s]}`,
			helmSecretJSON("sh.helm.release.v1.aws-lbc.v3", "aws-lbc", "kube-system", "pending-upgrade", "3", stale),
			helmSecretJSON("sh.helm.release.v1.karpenter.v2", "karpenter", "kube-system", "pending-install", "2", stale),
			helmSecretJSON("sh.helm.release.v1.keda.v7", "keda", "keda", "pending-upgrade", "7", fresh),
		)
	}))
	defer srv.Close()

	root := t.TempDir()
	writeRackVars(t, root, "myrack", map[string]string{
		"private_eks_host": srv.URL + "/proxy/rid-1234/private/eks/",
		"private_eks_user": "user-1",
		"private_eks_pass": "pass-1",
	})

	out := withTerraform(t, root, "myrack", func(t *testing.T, tf Terraform) {
		tf.reconcileStuckHelmReleases()
	})

	assert.Equal(t, []string{
		"/proxy/rid-1234/private/eks/api/v1/namespaces/kube-system/secrets/sh.helm.release.v1.aws-lbc.v3",
		"/proxy/rid-1234/private/eks/api/v1/namespaces/kube-system/secrets/sh.helm.release.v1.karpenter.v2",
	}, deleted)
	assert.Contains(t, out, "NOTICE: cleared stuck Helm release aws-lbc (pending-upgrade, revision 3) before apply")
	assert.Contains(t, out, "NOTICE: could not confirm clearing of stuck Helm release karpenter (pending-install, revision 2)")
	assert.NotContains(t, out, "cleared stuck Helm release karpenter")
	assert.NotContains(t, out, "keda")
}

func TestReconcileStuckHelmReleasesPrivateUnreachable(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	srv.Close()

	root := t.TempDir()
	writeRackVars(t, root, "myrack", map[string]string{
		"private_eks_host": srv.URL + "/proxy/rid-1234/private/eks/",
		"private_eks_user": "user-1",
		"private_eks_pass": "pass-1",
	})

	out := withTerraform(t, root, "myrack", func(t *testing.T, tf Terraform) {
		tf.reconcileStuckHelmReleases()
	})

	assert.Contains(t, out, "NOTICE: skipping stuck Helm release check, could not reach the cluster")
	assert.NotContains(t, out, srv.URL)
}

func TestReconcileStuckHelmReleasesPublicStaysQuiet(t *testing.T) {
	t.Setenv("PATH", "")

	root := t.TempDir()
	writeRackVars(t, root, "myrack", map[string]string{"region": "us-east-1"})

	out := withTerraform(t, root, "myrack", func(t *testing.T, tf Terraform) {
		tf.reconcileStuckHelmReleases()
	})

	assert.NotContains(t, out, "skipping stuck Helm release check")
	assert.NotContains(t, out, "NOTICE")
}

const (
	helmErrorTerraform15 = `
Error: could not get information about the resource: an error on the server ("") has prevented the request from succeeding (get customresourcedefinitions.apiextensions.k8s.io nodepools.karpenter.sh)

  with module.system.module.cluster.helm_release.karpenter_crd[0],
  on .terraform/modules/system/terraform/cluster/aws/karpenter.tf line 9, in resource "helm_release" "karpenter_crd":
   9: resource "helm_release" "karpenter_crd" {


Error: could not get information about the resource: an error on the server ("") has prevented the request from succeeding (get daemonsets.apps nvidia-device-plugin-mps-control-daemon)

  with module.system.module.cluster.helm_release.nvidia_device_plugin[0],
  on .terraform/modules/system/terraform/cluster/aws/nvidia.tf line 1, in resource "helm_release" "nvidia_device_plugin":
   1: resource "helm_release" "nvidia_device_plugin" {

`

	helmErrorTerraform013 = `
Error: unable to build kubernetes objects from new release manifest: unable to recognize "": an error on the server ("") has prevented the request from succeeding

  on .terraform/modules/system/terraform/cluster/aws/karpenter.tf line 34, in resource "helm_release" "karpenter":
  34: resource "helm_release" "karpenter" {


`

	helmErrorPending = `
Error: another operation (install/upgrade/rollback) is in progress

  with module.system.module.cluster.helm_release.aws_lbc,
  on .terraform/modules/system/terraform/cluster/aws/lbc.tf line 74, in resource "helm_release" "aws_lbc":
  74: resource "helm_release" "aws_lbc" {

`

	helmErrorKeda = `
Error: timed out waiting for the condition

  with module.system.module.cluster.helm_release.keda[0],
  on .terraform/modules/system/terraform/cluster/aws/keda.tf line 59, in resource "helm_release" "keda":
  59: resource "helm_release" "keda" {

`
)

const helmErrorUnreachable = `
Error: Kubernetes cluster unreachable: Get "https://example.org/version": EOF

  with module.system.module.cluster.helm_release.karpenter[0],
  on .terraform/modules/system/terraform/cluster/aws/karpenter.tf line 34, in resource "helm_release" "karpenter":
  34: resource "helm_release" "karpenter" {

`

func TestFailedHelmReleases(t *testing.T) {
	for _, tc := range []struct {
		name     string
		stderr   string
		want     []string
		installs []string
	}{
		{name: "empty"},
		{name: "terraform 1.5", stderr: helmErrorTerraform15, want: []string{"karpenter-crd", "nvidia-device-plugin"}},
		{name: "terraform 0.13", stderr: helmErrorTerraform013, want: []string{"karpenter"}},
		{name: "pending", stderr: helmErrorPending},
		{name: "pending beside another release", stderr: helmErrorPending + helmErrorKeda, want: []string{"keda"}},
		{
			name:   "pending vetoes a second block for the same release",
			stderr: helmErrorPending + strings.ReplaceAll(helmErrorKeda, "keda", "aws_lbc"),
		},
		{name: "cluster unreachable", stderr: helmErrorUnreachable, installs: []string{"karpenter"}},
		{
			name:   "pending vetoes cluster unreachable for the same release",
			stderr: helmErrorUnreachable + strings.ReplaceAll(helmErrorPending, "aws_lbc", "karpenter"),
		},
		{
			name:   "name in use",
			stderr: strings.ReplaceAll(helmErrorKeda, "timed out waiting for the condition", "cannot re-use a name that is still in use"),
		},
		{
			name:   "veto text wrapped across lines",
			stderr: strings.ReplaceAll(helmErrorKeda, "timed out waiting for the condition", "release failed\n\ncannot re-use a name that is\nstill in use"),
		},
		{
			name:   "state lock",
			stderr: "\nError: Error acquiring the state lock\n\nError message: HTTP remote state already locked\nLock Info:\n  ID:        1234\n",
		},
		{
			name:   "text before the first error",
			stderr: `debug: in resource "helm_release" "vpa"` + helmErrorKeda,
			want:   []string{"keda"},
		},
		{
			name:   "another resource type",
			stderr: "\nError: creating EKS Node Group\n\n  on main.tf line 1, in resource \"aws_eks_node_group\" \"cluster\":\n",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			failed, installs := failedHelmReleases(tc.stderr)

			assert.ElementsMatch(t, tc.want, slices.Collect(maps.Keys(failed)))
			assert.ElementsMatch(t, tc.installs, slices.Collect(maps.Keys(installs)))
		})
	}
}

func pendingSecretJSON(uid, release, namespace, status, version string, created time.Time) string {
	return fmt.Sprintf(`{"metadata":{"name":"sh.helm.release.v1.%s.v%s","namespace":%q,"uid":%q,"creationTimestamp":%q,"labels":{"owner":"helm","name":%q,"status":%q,"version":%q}},"type":%q}`,
		release, version, namespace, uid, created.UTC().Format(time.RFC3339), release, status, version, helmReleaseType)
}

const helmNoSecrets = " "

// helmAPI answers list call n with lists[n], repeating the last entry. An
// empty entry answers 502 and "hang" blocks until the request is abandoned.
type helmAPI struct {
	mu           sync.Mutex
	lists        []string
	deleteStatus int
	selectors    []string
	deleted      []string
	release      chan struct{}
}

func (a *helmAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	a.mu.Lock()

	if r.Method == "DELETE" {
		defer a.mu.Unlock()

		a.deleted = append(a.deleted, path.Base(r.URL.Path))

		if a.deleteStatus != 0 {
			w.WriteHeader(a.deleteStatus)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"kind":"Status","apiVersion":"v1","status":"Success"}`)

		return
	}

	a.selectors = append(a.selectors, r.URL.Query().Get("labelSelector"))
	items := a.lists[min(len(a.selectors), len(a.lists))-1]

	a.mu.Unlock()

	switch items {
	case "":
		w.WriteHeader(http.StatusBadGateway)
	case "hang":
		select {
		case <-r.Context().Done():
		case <-a.release:
		}
	default:
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"kind":"SecretList","apiVersion":"v1","items":[%s]}`, items)
	}
}

func (a *helmAPI) calls() ([]string, []string) {
	a.mu.Lock()
	defer a.mu.Unlock()

	return append([]string{}, a.selectors...), append([]string{}, a.deleted...)
}

// applyWithHelmAPI runs apply() on a private rack whose cluster is api, with
// terraform writing stderr and returning applyErr.
func applyWithHelmAPI(t *testing.T, api *helmAPI, stderr string, applyErr error) (string, error) {
	t.Helper()

	api.release = make(chan struct{})

	srv := httptest.NewTLSServer(api)
	t.Cleanup(srv.Close)
	t.Cleanup(func() { close(api.release) })

	root := t.TempDir()
	writeRackVars(t, root, "myrack", map[string]string{
		"private_eks_host": srv.URL + "/proxy/rid-1234/private/eks/",
		"private_eks_user": "user-1",
		"private_eks_pass": "pass-1",
	})

	stubTerraform(t, func(_, w io.Writer, _ ...string) error {
		fmt.Fprint(w, stderr)
		return applyErr
	})

	var err error
	out := withTerraform(t, root, "myrack", func(t *testing.T, tf Terraform) {
		err = tf.apply()
	})

	return out, err
}

func shortHelmClear(t *testing.T, interval, budget time.Duration) {
	t.Helper()

	origInterval, origBudget := helmClearInterval, helmClearBudget
	helmClearInterval, helmClearBudget = interval, budget
	t.Cleanup(func() { helmClearInterval, helmClearBudget = origInterval, origBudget })
}

var errApplyFailed = errors.New("exit status 1")

func TestApplyClearsHelmReleasesLeftByFailedApply(t *testing.T) {
	fresh := time.Now().Add(-2 * time.Minute)

	keda := pendingSecretJSON("uid-keda", "keda", "keda", "pending-upgrade", "7", fresh)

	api := &helmAPI{lists: []string{
		keda,
		strings.Join([]string{
			keda,
			pendingSecretJSON("uid-nvidia", "nvidia-device-plugin", "kube-system", "pending-upgrade", "4", time.Now()),
			pendingSecretJSON("uid-karpenter", "karpenter", "kube-system", "pending-upgrade", "9", time.Now()),
			pendingSecretJSON("uid-lbc", "aws-lbc", "kube-system", "pending-upgrade", "3", time.Now()),
			pendingSecretJSON("uid-datadog", "datadog", "monitoring", "pending-upgrade", "2", time.Now()),
			pendingSecretJSON("uid-other-ns", "nvidia-device-plugin", "default", "pending-upgrade", "1", time.Now()),
		}, ","),
	}}

	stderr := helmErrorTerraform15 + helmErrorPending + helmErrorKeda + strings.ReplaceAll(helmErrorKeda, "keda", "datadog")

	out, err := applyWithHelmAPI(t, api, stderr, errApplyFailed)

	assert.Equal(t, errApplyFailed, err)

	selectors, deleted := api.calls()
	assert.Equal(t, []string{helmPendingSelector, helmPendingSelector}, selectors)
	assert.Equal(t, []string{"sh.helm.release.v1.nvidia-device-plugin.v4"}, deleted)

	cleared := "NOTICE: cleared Helm release nvidia-device-plugin left pending-upgrade by the failed apply (revision 4)\n"
	recent := "WARNING: Helm release keda is pending-upgrade (revision 7) and under 15 minutes old, so it was not cleared. A rack update that changes it fails until then.\n"

	assert.Equal(t, stderr+cleared+recent, out)
}

func TestApplyClearsPendingInstallAfterClusterUnreachable(t *testing.T) {
	api := &helmAPI{lists: []string{
		helmNoSecrets,
		strings.Join([]string{
			pendingSecretJSON("uid-install", "karpenter", "kube-system", "pending-install", "1", time.Now()),
			pendingSecretJSON("uid-upgrade", "karpenter", "kube-system", "pending-upgrade", "2", time.Now()),
		}, ","),
	}}

	out, err := applyWithHelmAPI(t, api, helmErrorUnreachable, errApplyFailed)

	assert.Equal(t, errApplyFailed, err)

	_, deleted := api.calls()
	assert.Equal(t, []string{"sh.helm.release.v1.karpenter.v1"}, deleted)
	assert.Contains(t, out, "NOTICE: cleared Helm release karpenter left pending-install by the failed apply (revision 1)\n")
}

func TestApplyHelmCleanupIsAwsOnly(t *testing.T) {
	api := &helmAPI{release: make(chan struct{}), lists: []string{
		pendingSecretJSON("uid-nvidia", "nvidia-device-plugin", "kube-system", "pending-upgrade", "4", time.Now()),
	}}

	srv := httptest.NewTLSServer(api)
	defer srv.Close()

	root := t.TempDir()
	writeRackVars(t, root, "myrack", map[string]string{
		"private_eks_host": srv.URL + "/proxy/rid-1234/private/eks/",
		"private_eks_pass": "pass-1",
	})

	stubTerraform(t, func(_, w io.Writer, _ ...string) error {
		fmt.Fprint(w, helmErrorTerraform15)
		return errApplyFailed
	})

	var err error
	out := withTerraform(t, root, "myrack", func(t *testing.T, tf Terraform) {
		tf.provider = "azure"
		err = tf.apply()
	})

	assert.Equal(t, errApplyFailed, err)
	assert.Equal(t, helmErrorTerraform15, out)

	selectors, deleted := api.calls()
	assert.Empty(t, selectors)
	assert.Empty(t, deleted)
}

func TestApplyClearsReusedHelmRevision(t *testing.T) {
	api := &helmAPI{lists: []string{
		pendingSecretJSON("uid-a", "nvidia-device-plugin", "kube-system", "pending-upgrade", "7", time.Now().Add(-1*time.Hour)),
		pendingSecretJSON("uid-b", "nvidia-device-plugin", "kube-system", "pending-upgrade", "7", time.Now()),
	}}

	out, err := applyWithHelmAPI(t, api, helmErrorTerraform15, errApplyFailed)

	assert.Equal(t, errApplyFailed, err)

	_, deleted := api.calls()
	assert.Equal(t, []string{"sh.helm.release.v1.nvidia-device-plugin.v7", "sh.helm.release.v1.nvidia-device-plugin.v7"}, deleted)
	assert.Contains(t, out, "NOTICE: cleared stuck Helm release nvidia-device-plugin (pending-upgrade, revision 7) before apply\n")
	assert.Contains(t, out, "NOTICE: cleared Helm release nvidia-device-plugin left pending-upgrade by the failed apply (revision 7)\n")
}

func TestApplyKeepsHelmRecordSeenBeforeApply(t *testing.T) {
	ahead := pendingSecretJSON("uid-nvidia", "nvidia-device-plugin", "kube-system", "pending-upgrade", "4", time.Now().Add(30*time.Second))

	api := &helmAPI{lists: []string{ahead}}

	out, err := applyWithHelmAPI(t, api, helmErrorTerraform15, errApplyFailed)

	assert.Equal(t, errApplyFailed, err)

	selectors, deleted := api.calls()
	assert.Len(t, selectors, 2)
	assert.Empty(t, deleted)
	assert.Contains(t, out, "WARNING: Helm release nvidia-device-plugin is pending-upgrade (revision 4) and under 15 minutes old")
}

func TestApplyHelmCleanupFallsBackToCreationTime(t *testing.T) {
	api := &helmAPI{lists: []string{
		"",
		strings.Join([]string{
			pendingSecretJSON("uid-old", "karpenter-crd", "kube-system", "pending-upgrade", "2", time.Now().Add(-10*time.Minute)),
			pendingSecretJSON("uid-new", "nvidia-device-plugin", "kube-system", "pending-upgrade", "4", time.Now().Add(time.Minute)),
		}, ","),
	}}

	_, err := applyWithHelmAPI(t, api, helmErrorTerraform15, errApplyFailed)

	assert.Equal(t, errApplyFailed, err)

	_, deleted := api.calls()
	assert.Equal(t, []string{"sh.helm.release.v1.nvidia-device-plugin.v4"}, deleted)
}

func TestApplyHelmCleanupRetriesUntilTheClusterAnswers(t *testing.T) {
	shortHelmClear(t, 10*time.Millisecond, 5*time.Second)

	api := &helmAPI{lists: []string{
		helmNoSecrets,
		"",
		"",
		pendingSecretJSON("uid-nvidia", "nvidia-device-plugin", "kube-system", "pending-upgrade", "4", time.Now()),
	}}

	out, err := applyWithHelmAPI(t, api, helmErrorTerraform15, errApplyFailed)

	assert.Equal(t, errApplyFailed, err)

	selectors, deleted := api.calls()
	assert.Len(t, selectors, 4)
	assert.Equal(t, []string{"sh.helm.release.v1.nvidia-device-plugin.v4"}, deleted)
	assert.Equal(t, 1, strings.Count(out, "NOTICE: retrying for up to 5s to clear Helm releases the failed apply may have left pending\n"))
	assert.NotContains(t, out, "could not clear")
}

const helmGaveUp = "NOTICE: could not clear Helm releases the failed apply may have left pending. A rerun can fail on them for up to 15 minutes.\n"

func TestApplyHelmCleanupGivesUpAfterBudget(t *testing.T) {
	for _, answer := range []string{"", "hang"} {
		t.Run("list "+answer, func(t *testing.T) {
			shortHelmClear(t, 20*time.Millisecond, 300*time.Millisecond)

			api := &helmAPI{lists: []string{helmNoSecrets, answer}}

			start := time.Now()
			out, err := applyWithHelmAPI(t, api, helmErrorTerraform15, errApplyFailed)

			assert.Equal(t, errApplyFailed, err)
			assert.Less(t, time.Since(start), 5*time.Second)

			_, deleted := api.calls()
			assert.Empty(t, deleted)
			assert.Equal(t, 1, strings.Count(out, helmGaveUp))
		})
	}
}

func TestApplyHelmCleanupRetriesFailedDelete(t *testing.T) {
	shortHelmClear(t, 20*time.Millisecond, 300*time.Millisecond)

	api := &helmAPI{
		deleteStatus: http.StatusBadGateway,
		lists: []string{
			helmNoSecrets,
			pendingSecretJSON("uid-nvidia", "nvidia-device-plugin", "kube-system", "pending-upgrade", "4", time.Now()),
		},
	}

	out, err := applyWithHelmAPI(t, api, helmErrorTerraform15, errApplyFailed)

	assert.Equal(t, errApplyFailed, err)

	_, deleted := api.calls()
	assert.Greater(t, len(deleted), 1)
	assert.NotContains(t, out, "cleared Helm release")
	assert.Equal(t, 1, strings.Count(out, helmGaveUp))
}

func TestApplyHelmCleanupCountsMissingRecordAsCleared(t *testing.T) {
	shortHelmClear(t, 10*time.Millisecond, time.Second)

	api := &helmAPI{
		deleteStatus: http.StatusNotFound,
		lists: []string{
			helmNoSecrets,
			pendingSecretJSON("uid-nvidia", "nvidia-device-plugin", "kube-system", "pending-upgrade", "4", time.Now()),
		},
	}

	out, err := applyWithHelmAPI(t, api, helmErrorTerraform15, errApplyFailed)

	assert.Equal(t, errApplyFailed, err)
	assert.Contains(t, out, "NOTICE: cleared Helm release nvidia-device-plugin left pending-upgrade by the failed apply (revision 4)\n")
	assert.NotContains(t, out, "could not clear")
}

func TestApplyHelmCleanupSkips(t *testing.T) {
	pending := pendingSecretJSON("uid-nvidia", "nvidia-device-plugin", "kube-system", "pending-upgrade", "4", time.Now())
	recent := pendingSecretJSON("uid-keda", "keda", "keda", "pending-upgrade", "7", time.Now().Add(-2*time.Minute))

	for _, tc := range []struct {
		name   string
		stderr string
		err    error
	}{
		{name: "apply succeeded", stderr: ""},
		{name: "no helm error", stderr: "\nError: Error acquiring the state lock\n\nError message: HTTP remote state already locked\n", err: errApplyFailed},
		{name: "only the pending error", stderr: helmErrorPending, err: errApplyFailed},
		{name: "interrupted", stderr: helmErrorTerraform15, err: errTerraformInterrupted},
		{name: "stderr over the capture limit", stderr: helmErrorTerraform15 + strings.Repeat("x", stderrCaptureMax), err: errApplyFailed},
	} {
		t.Run(tc.name, func(t *testing.T) {
			api := &helmAPI{lists: []string{recent, recent + "," + pending}}

			out, err := applyWithHelmAPI(t, api, tc.stderr, tc.err)

			assert.Equal(t, tc.err, err)

			selectors, deleted := api.calls()
			assert.Len(t, selectors, 1)
			assert.Empty(t, deleted)
			assert.NotContains(t, out, "NOTICE")
			assert.Equal(t, tc.err != nil, strings.Contains(out, "WARNING: Helm release keda is pending-upgrade"))
		})
	}
}

func TestApplyHelmCleanupPublicRackWithoutAwsStaysQuiet(t *testing.T) {
	t.Setenv("PATH", "")

	root := t.TempDir()
	writeRackVars(t, root, "myrack", map[string]string{"region": "us-east-1"})

	stubTerraform(t, func(_, w io.Writer, _ ...string) error {
		fmt.Fprint(w, helmErrorTerraform15)
		return errApplyFailed
	})

	var err error
	out := withTerraform(t, root, "myrack", func(t *testing.T, tf Terraform) {
		err = tf.apply()
	})

	assert.Equal(t, errApplyFailed, err)
	assert.Equal(t, helmErrorTerraform15, out)
}
