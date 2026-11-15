package rack

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/convox/convox/pkg/common"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const helmReleaseType = "helm.sh/release.v1"

const helmPendingSelector = "owner=helm,status in (pending-install,pending-upgrade,pending-rollback)"

// Only clear releases pending longer than most live helm ops: system releases time
// out within 600s (contour 3600s), so a fresh pending secret may be an in-flight apply.
const helmStuckMinAge = 15 * time.Minute

var helmPendingStatuses = map[string]bool{
	"pending-install":  true,
	"pending-upgrade":  true,
	"pending-rollback": true,
}

var (
	helmClearInterval = 5 * time.Second
	helmClearBudget   = 60 * time.Second
)

// Helm returns these before it writes a release record.
var helmNoRecordErrors = []string{
	"another operation (install/upgrade/rollback) is in progress",
	"cannot re-use a name that is still in use",
}

// The helm provider can return this in place of a failed install's own error.
const helmUnreachableError = "Kubernetes cluster unreachable"

var helmReleaseResource = regexp.MustCompile(`resource "helm_release" "([a-z0-9_]+)"`)

func convoxHelmReleases(rack string) map[string]string {
	system := strings.ToLower(rack) + "-system"
	return map[string]string{
		"aws-lbc":              "kube-system",
		"karpenter":            "kube-system",
		"karpenter-crd":        "kube-system",
		"keda":                 "keda",
		"vpa":                  "vpa",
		"dcgm-exporter":        "kube-system",
		"nvidia-device-plugin": "kube-system",
		"contour":              system,
		"contour-internal":     system,
	}
}

type helmReleaseSecret struct {
	secretName string
	release    string
	namespace  string
	status     string
	version    string
	created    time.Time
	uid        types.UID
}

func pendingHelmSecrets(found []helmReleaseSecret, allow map[string]string) []helmReleaseSecret {
	var out []helmReleaseSecret
	for _, s := range found {
		if ns, ok := allow[s.release]; !ok || ns != s.namespace {
			continue
		}
		if !helmPendingStatuses[s.status] {
			continue
		}
		out = append(out, s)
	}
	return out
}

// stuckHelmSecrets returns the allowlisted release secrets stuck pending-* past
// minAge. Helm blocks new ops while pending, so no max-revision math is needed.
func stuckHelmSecrets(found []helmReleaseSecret, allow map[string]string, now time.Time, minAge time.Duration) []helmReleaseSecret {
	var out []helmReleaseSecret
	for _, s := range pendingHelmSecrets(found, allow) {
		if now.Sub(s.created) < minAge {
			continue
		}
		out = append(out, s)
	}
	return out
}

// reconcileStuckHelmReleases clears convox-owned Helm releases stranded in
// pending-* by an interrupted apply. Best-effort: it never fails an apply.
func (t Terraform) reconcileStuckHelmReleases() (seen map[types.UID]bool, recent string) {
	if t.provider != "aws" {
		return nil, ""
	}

	vars, err := t.vars()
	if err != nil {
		return nil, ""
	}

	// the console job that applies private racks cannot set CONVOX_DEBUG
	privateEKS := vars["private_eks_host"] != ""

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cluster := strings.ToLower(t.name)

	client, err := eksClient(ctx, cluster, vars)
	if err != nil {
		helmReconcileDebug("build eks client for %s: %v", cluster, err)
		return nil, ""
	}

	found, err := listPendingHelmSecrets(ctx, client)
	if err != nil {
		if privateEKS {
			fmt.Fprintln(os.Stderr, "NOTICE: skipping stuck Helm release check, could not reach the cluster")
		}
		helmReconcileDebug("list helm secrets: %v", err)
		return nil, ""
	}

	allow := convoxHelmReleases(t.name)
	now := time.Now()

	seen = map[types.UID]bool{}

	for _, s := range pendingHelmSecrets(found, allow) {
		seen[s.uid] = true

		if now.Sub(s.created) < helmStuckMinAge {
			recent += fmt.Sprintf("WARNING: Helm release %s is %s (revision %s) and under 15 minutes old, so it was not cleared. A rack update that changes it fails until then.\n", s.release, s.status, s.version)
		}
	}

	for _, s := range stuckHelmSecrets(found, allow, now, helmStuckMinAge) {
		if err := client.CoreV1().Secrets(s.namespace).Delete(ctx, s.secretName, metav1.DeleteOptions{}); err != nil {
			if privateEKS {
				fmt.Fprintf(os.Stderr, "NOTICE: could not confirm clearing of stuck Helm release %s (%s, revision %s)\n", s.release, s.status, s.version)
			}
			helmReconcileDebug("delete %s/%s: %v", s.namespace, s.secretName, err)
			continue
		}

		fmt.Fprintf(os.Stderr, "NOTICE: cleared stuck Helm release %s (%s, revision %s) before apply\n", s.release, s.status, s.version)
	}

	return seen, recent
}

func listPendingHelmSecrets(ctx context.Context, client kubernetes.Interface) ([]helmReleaseSecret, error) {
	list, err := client.CoreV1().Secrets(metav1.NamespaceAll).List(ctx, metav1.ListOptions{LabelSelector: helmPendingSelector})
	if err != nil {
		return nil, err
	}

	var found []helmReleaseSecret
	for i := range list.Items {
		s := &list.Items[i]
		if string(s.Type) != helmReleaseType {
			continue
		}
		found = append(found, helmReleaseSecret{
			secretName: s.Name,
			release:    s.Labels["name"],
			namespace:  s.Namespace,
			status:     s.Labels["status"],
			version:    s.Labels["version"],
			created:    s.CreationTimestamp.Time,
			uid:        s.UID,
		})
	}

	return found, nil
}

func failedHelmReleases(stderr string) (failed, installs map[string]bool) {
	failed, installs = map[string]bool{}, map[string]bool{}
	vetoed := map[string]bool{}

	for _, block := range strings.Split("\n"+stderr, "\nError: ")[1:] {
		block = strings.Join(strings.Fields(block), " ")

		m := helmReleaseResource.FindStringSubmatch(block)
		if m == nil {
			continue
		}

		release := strings.ReplaceAll(m[1], "_", "-")

		switch {
		case slices.ContainsFunc(helmNoRecordErrors, func(text string) bool { return strings.Contains(block, text) }):
			vetoed[release] = true
		case strings.Contains(block, helmUnreachableError):
			installs[release] = true
		default:
			failed[release] = true
		}
	}

	for release := range vetoed {
		delete(failed, release)
		delete(installs, release)
	}

	return failed, installs
}

// clearFailedHelmReleases is best-effort: it never fails an apply.
func (t Terraform) clearFailedHelmReleases(seen map[types.UID]bool, start time.Time, stderr string) {
	if t.provider != "aws" {
		return
	}

	failed, installs := failedHelmReleases(stderr)
	if len(failed)+len(installs) == 0 {
		return
	}

	vars, err := t.vars()
	if err != nil {
		return
	}

	deadline := time.Now().Add(helmClearBudget)
	waiting := false

	for !t.clearFailedHelmReleasesOnce(vars, failed, installs, seen, start, deadline) {
		if time.Until(deadline) < helmClearInterval {
			fmt.Fprintln(os.Stderr, "NOTICE: could not clear Helm releases the failed apply may have left pending. A rerun can fail on them for up to 15 minutes.")
			return
		}

		if !waiting {
			fmt.Fprintf(os.Stderr, "NOTICE: retrying for up to %s to clear Helm releases the failed apply may have left pending\n", helmClearBudget)
			waiting = true
		}

		time.Sleep(helmClearInterval)
	}
}

func (t Terraform) clearFailedHelmReleasesOnce(vars map[string]string, failed, installs map[string]bool, seen map[types.UID]bool, start, deadline time.Time) bool {
	ctx, cancel := context.WithTimeout(context.Background(), min(30*time.Second, time.Until(deadline)))
	defer cancel()

	cluster := strings.ToLower(t.name)

	client, err := eksClient(ctx, cluster, vars)
	if err != nil {
		helmReconcileDebug("build eks client for %s: %v", cluster, err)
		return ctx.Err() == nil
	}

	found, err := listPendingHelmSecrets(ctx, client)
	if err != nil {
		helmReconcileDebug("list helm secrets: %v", err)
		return false
	}

	done := true

	for _, s := range pendingHelmSecrets(found, convoxHelmReleases(t.name)) {
		if !failed[s.release] && (!installs[s.release] || s.status != "pending-install") {
			continue
		}

		// creation timestamps have one second resolution
		if seen[s.uid] || (seen == nil && s.created.Before(start.Truncate(time.Second))) {
			continue
		}

		err := client.CoreV1().Secrets(s.namespace).Delete(ctx, s.secretName, metav1.DeleteOptions{})
		if err != nil && !apierrors.IsNotFound(err) {
			helmReconcileDebug("delete %s/%s: %v", s.namespace, s.secretName, err)
			done = false
			continue
		}

		fmt.Fprintf(os.Stderr, "NOTICE: cleared Helm release %s left %s by the failed apply (revision %s)\n", s.release, s.status, s.version)
	}

	return done
}

func eksClient(ctx context.Context, cluster string, vars map[string]string) (*kubernetes.Clientset, error) {
	if host := vars["private_eks_host"]; host != "" {
		// client-go only installs basic auth when Username is set
		user := common.CoalesceString(vars["private_eks_user"], "convox")

		return kubernetes.NewForConfig(&rest.Config{
			Host:            host,
			Username:        user,
			Password:        vars["private_eks_pass"],
			TLSClientConfig: rest.TLSClientConfig{Insecure: true},
			Timeout:         20 * time.Second,
		})
	}

	region := vars["region"]

	endpoint, ca, err := eksClusterEndpoint(ctx, cluster, region)
	if err != nil {
		return nil, err
	}

	token, err := eksToken(ctx, cluster, region)
	if err != nil {
		return nil, err
	}

	return kubernetes.NewForConfig(&rest.Config{
		Host:            endpoint,
		BearerToken:     token,
		TLSClientConfig: rest.TLSClientConfig{CAData: ca},
		Timeout:         20 * time.Second,
	})
}

func eksClusterEndpoint(ctx context.Context, cluster, region string) (string, []byte, error) {
	out, err := awsEKS(ctx, region, "describe-cluster", "--name", cluster, "--output", "json")
	if err != nil {
		return "", nil, err
	}

	var resp struct {
		Cluster struct {
			Endpoint             string `json:"endpoint"`
			CertificateAuthority struct {
				Data string `json:"data"`
			} `json:"certificateAuthority"`
		} `json:"cluster"`
	}
	if err := json.Unmarshal(out, &resp); err != nil {
		return "", nil, err
	}

	ca, err := base64.StdEncoding.DecodeString(resp.Cluster.CertificateAuthority.Data)
	if err != nil {
		return "", nil, err
	}
	if resp.Cluster.Endpoint == "" || len(ca) == 0 {
		return "", nil, fmt.Errorf("empty cluster endpoint or ca")
	}

	return resp.Cluster.Endpoint, ca, nil
}

func eksToken(ctx context.Context, cluster, region string) (string, error) {
	out, err := awsEKS(ctx, region, "get-token", "--cluster-name", cluster, "--output", "json")
	if err != nil {
		return "", err
	}

	var resp struct {
		Status struct {
			Token string `json:"token"`
		} `json:"status"`
	}
	if err := json.Unmarshal(out, &resp); err != nil {
		return "", err
	}
	if resp.Status.Token == "" {
		return "", fmt.Errorf("empty eks token")
	}

	return resp.Status.Token, nil
}

func awsEKS(ctx context.Context, region string, args ...string) ([]byte, error) {
	full := append([]string{"eks"}, args...)
	if region != "" {
		full = append(full, "--region", region)
	}

	out, err := exec.CommandContext(ctx, "aws", full...).Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("%v: %s", err, strings.TrimSpace(string(ee.Stderr)))
		}
		return nil, err
	}

	return out, nil
}

func helmReconcileDebug(format string, args ...interface{}) {
	if os.Getenv("CONVOX_DEBUG") == "true" {
		fmt.Fprintf(os.Stderr, "debug: helm reconcile: "+format+"\n", args...)
	}
}
