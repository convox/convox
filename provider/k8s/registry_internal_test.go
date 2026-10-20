package k8s

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/convox/convox/pkg/mock"
	"github.com/convox/stdapi"
	"github.com/stretchr/testify/require"
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

type artifactRegistryEngine struct {
	*mock.TestEngine
	host string
}

func (e artifactRegistryEngine) RepositoryAuth(string) (string, string, error) {
	return "_json_key", "key1", nil
}

func (e artifactRegistryEngine) RepositoryHost(app string) (string, bool, error) {
	return e.host + "/project1/rack1/" + app, true, nil
}

func (e artifactRegistryEngine) RepositoryPrefix() string {
	return "project1/rack1/"
}

func TestArtifactUploadPath(t *testing.T) {
	require.True(t, artifactUploadPath("project1/rack1/", "/v2/project1/rack1/pkg/blobs/uploads/abc123"))
	require.True(t, artifactUploadPath("project1/rack1/", "/v2/project1/rack1/pkg/blobs/uploads/artifacts-uploads/abc123"))

	require.False(t, artifactUploadPath("project1/rack1/", "/v2/project1/rack1/pkg/blobs/uploads/"))
	require.False(t, artifactUploadPath("project1/rack1/", "/v2/project1/rack1/app1/blobs/uploads/abc123"))
	require.False(t, artifactUploadPath("project1/rack1/", "/v2/project1/rack2/pkg/blobs/uploads/abc123"))
	require.False(t, artifactUploadPath("project1/rack1/", "/artifacts-uploads/namespaces/project1/repositories/rack1/uploads/abc123"))

	require.False(t, artifactUploadPath("rack1/", "/v2/rack1/pkg/blobs/uploads/abc123"))
	require.False(t, artifactUploadPath("", "/v2/project1/rack1/pkg/blobs/uploads/abc123"))
}

func TestArtifactUploadLocation(t *testing.T) {
	require.Equal(t, "/v2/project1/rack1/pkg/blobs/uploads/artifacts-uploads/abc123?upload_id=x",
		artifactUploadLocation("project1/rack1/", "/artifacts-uploads/namespaces/project1/repositories/rack1/uploads/abc123?upload_id=x"))

	for _, l := range []string{
		"/v2/project1/rack1/pkg/blobs/uploads/abc123?state=s",
		"/artifacts-uploads/namespaces/project1/repositories/rack2/uploads/abc123",
	} {
		require.Equal(t, l, artifactUploadLocation("project1/rack1/", l))
	}

	require.Equal(t, "/artifacts-uploads/namespaces/rack1/uploads/abc123", artifactUploadLocation("rack1/", "/artifacts-uploads/namespaces/rack1/uploads/abc123"))
}

func TestRegistryProxyArtifactUpload(t *testing.T) {
	var upstream *http.Request

	ar := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstream = r

		if strings.HasPrefix(r.URL.Path, "/artifacts-uploads/") {
			w.Header().Set("Location", "/v2/project1/rack1/pkg/blobs/uploads/abc123?state=s")
		} else {
			w.Header().Set("Location", "/artifacts-uploads/namespaces/project1/repositories/rack1/uploads/def456?upload_id=y")
		}

		w.WriteHeader(http.StatusAccepted)
	}))
	defer ar.Close()

	p := &Provider{Engine: artifactRegistryEngine{TestEngine: &mock.TestEngine{}, host: strings.TrimPrefix(ar.URL, "https://")}}

	tests := []struct {
		path     string
		upstream string
		location string
	}{
		{
			"/v2/project1/rack1/pkg/blobs/uploads/artifacts-uploads/abc123?upload_id=x",
			"/artifacts-uploads/namespaces/project1/repositories/rack1/uploads/abc123?upload_id=x",
			"/v2/project1/rack1/pkg/blobs/uploads/abc123?state=s",
		},
		{
			"/v2/project1/rack1/pkg/blobs/uploads/abc123?state=s",
			"/v2/project1/rack1/pkg/blobs/uploads/abc123?state=s",
			"/v2/project1/rack1/pkg/blobs/uploads/artifacts-uploads/def456?upload_id=y",
		},
	}

	for _, tt := range tests {
		w := httptest.NewRecorder()

		require.NoError(t, p.RegistryProxy(stdapi.NewContext(w, httptest.NewRequest(http.MethodPatch, tt.path, nil))))
		require.Equal(t, http.StatusAccepted, w.Code)
		require.Equal(t, tt.location, w.Header().Get("Location"))

		require.Equal(t, tt.upstream, upstream.URL.RequestURI())

		user, pass, ok := upstream.BasicAuth()
		require.True(t, ok)
		require.Equal(t, "_json_key", user)
		require.Equal(t, "key1", pass)
	}
}
