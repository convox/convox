package k8s

import (
	"testing"

	"github.com/convox/convox/pkg/mock"
)

func TestRegistryAppRejectsReservedNames(t *testing.T) {
	p := &Provider{Engine: &mock.TestEngine{}}

	for _, path := range []string{"system/manifests/latest", "rack/blobs/sha256:abc", "system"} {
		app, err := p.registryApp(path)
		if err == nil {
			t.Fatalf("registryApp(%q) = %q, want error", path, app)
		}

		if err.Error() != "app name is reserved" {
			t.Fatalf("registryApp(%q) error = %q, want %q", path, err.Error(), "app name is reserved")
		}
	}
}
