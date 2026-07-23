package rack

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const helmReleaseType = "helm.sh/release.v1"

// Only clear releases pending longer than a live helm op could be: the system
// releases pin timeout=600, so a fresh pending secret may be an in-flight apply.
const helmStuckMinAge = 15 * time.Minute

var helmPendingStatuses = map[string]bool{
	"pending-install":  true,
	"pending-upgrade":  true,
	"pending-rollback": true,
}

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
}

// stuckHelmSecrets returns the allowlisted release secrets stuck pending-* past
// minAge. Helm blocks new ops while pending, so no max-revision math is needed.
func stuckHelmSecrets(found []helmReleaseSecret, allow map[string]string, now time.Time, minAge time.Duration) []helmReleaseSecret {
	var out []helmReleaseSecret
	for _, s := range found {
		if ns, ok := allow[s.release]; !ok || ns != s.namespace {
			continue
		}
		if !helmPendingStatuses[s.status] {
			continue
		}
		if now.Sub(s.created) < minAge {
			continue
		}
		out = append(out, s)
	}
	return out
}

// reconcileStuckHelmReleases clears convox-owned Helm releases stranded in
// pending-* by an interrupted apply. Best-effort: failures log under CONVOX_DEBUG.
func (t Terraform) reconcileStuckHelmReleases() {
	if t.provider != "aws" {
		return
	}

	vars, err := t.vars()
	if err != nil {
		return
	}
	if vars["private_eks_host"] != "" {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cluster := strings.ToLower(t.name)

	client, err := eksClient(ctx, cluster, vars["region"])
	if err != nil {
		helmReconcileDebug("build eks client for %s: %v", cluster, err)
		return
	}

	list, err := client.CoreV1().Secrets(metav1.NamespaceAll).List(ctx, metav1.ListOptions{LabelSelector: "owner=helm"})
	if err != nil {
		helmReconcileDebug("list helm secrets: %v", err)
		return
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
		})
	}

	for _, s := range stuckHelmSecrets(found, convoxHelmReleases(t.name), time.Now(), helmStuckMinAge) {
		fmt.Fprintf(os.Stderr, "NOTICE: clearing stuck Helm release %s (%s, revision %s) before apply\n", s.release, s.status, s.version)
		if err := client.CoreV1().Secrets(s.namespace).Delete(ctx, s.secretName, metav1.DeleteOptions{}); err != nil {
			helmReconcileDebug("delete %s/%s: %v", s.namespace, s.secretName, err)
		}
	}
}

func eksClient(ctx context.Context, cluster, region string) (*kubernetes.Clientset, error) {
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
