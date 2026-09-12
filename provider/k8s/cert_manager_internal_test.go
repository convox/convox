package k8s

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/version"
)

func TestCertManagerCurrent(t *testing.T) {
	for _, c := range []struct {
		name   string
		images []string
		want   bool
	}{
		{"no deployment", nil, false},
		{"v1.10", []string{"quay.io/jetstack/cert-manager-controller:v1.10.2"}, false},
		{"v1.21.1", []string{CERT_MANAGER_IMAGE}, true},
		{"digest", []string{"quay.io/jetstack/cert-manager-controller@sha256:abc"}, false},
		{"registry port", []string{"registry.example.com:5000/jetstack/cert-manager-controller:v1.21.1"}, false},
		{"sidecar", []string{"busybox:latest", CERT_MANAGER_IMAGE}, true},
	} {
		t.Run(c.name, func(t *testing.T) {
			cs := []corev1.Container{}
			for _, i := range c.images {
				cs = append(cs, corev1.Container{Name: "cert-manager", Image: i})
			}

			if got := certManagerCurrent(cs); got != c.want {
				t.Fatalf("certManagerCurrent(%v) = %v, want %v", c.images, got, c.want)
			}
		})
	}
}

func TestKubernetesAtLeast(t *testing.T) {
	for _, c := range []struct {
		major string
		minor string
		want  bool
	}{
		{"1", "29", false},
		{"1", "30", true},
		{"1", "31+", true},
		{"1", "35", true},
		{"2", "0", true},
		{"", "", true},
		{"1", "v1.29", true},
	} {
		if got := kubernetesAtLeast(&version.Info{Major: c.major, Minor: c.minor}, 1, CERT_MANAGER_MIN_MINOR); got != c.want {
			t.Fatalf("kubernetesAtLeast(%q.%q) = %v, want %v", c.major, c.minor, got, c.want)
		}
	}
}
