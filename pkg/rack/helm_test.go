package rack

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
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
