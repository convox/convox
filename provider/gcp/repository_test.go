package gcp

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRepositoryPrefix(t *testing.T) {
	p := &Provider{Registry: "europe-west6-docker.pkg.dev/project1/rack1"}

	require.Equal(t, "project1/rack1/", p.RepositoryPrefix())

	host, remote, err := p.RepositoryHost("app1")
	require.NoError(t, err)
	require.True(t, remote)
	require.Equal(t, "europe-west6-docker.pkg.dev/"+p.RepositoryPrefix()+"app1", host)
}

func TestRepositoryPrefixWithoutPath(t *testing.T) {
	p := &Provider{Registry: "registry.example.com"}

	require.Equal(t, "", p.RepositoryPrefix())
}
