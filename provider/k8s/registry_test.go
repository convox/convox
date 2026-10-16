package k8s

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestArtifactUploadPath(t *testing.T) {
	require.True(t, artifactUploadPath("project1/rack1/", "/artifacts-uploads/namespaces/project1/repositories/rack1/uploads/abc123"))
	require.True(t, artifactUploadPath("project1/rack1/", "/v2/project1/rack1/pkg/blobs/uploads/abc123"))

	require.False(t, artifactUploadPath("project1/rack1/", "/artifacts-uploads/namespaces/project2/repositories/rack1/uploads/abc123"))
	require.False(t, artifactUploadPath("project1/rack1/", "/artifacts-uploads/namespaces/project1/repositories/rack2/uploads/abc123"))
	require.False(t, artifactUploadPath("project1/rack1/", "/artifacts-uploads/namespaces/project1/repositories/rack1/other"))
	require.False(t, artifactUploadPath("project1/rack1/", "/v2/project1/rack1/app1/blobs/uploads/abc123"))
	require.False(t, artifactUploadPath("project1/rack1/", "/v2/project1/rack2/pkg/blobs/uploads/abc123"))

	require.False(t, artifactUploadPath("rack1/", "/v2/rack1/pkg/blobs/uploads/abc123"))
	require.False(t, artifactUploadPath("", "/artifacts-uploads/namespaces/project1/repositories/rack1/uploads/abc123"))
}
