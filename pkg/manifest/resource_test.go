package manifest

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestResourceProviderClassification(t *testing.T) {
	for _, tc := range []struct {
		rType         string
		provider      string
		rds           bool
		elasticache   bool
		containerized bool
		custom        bool
	}{
		{rType: "postgres", containerized: true},
		{rType: "postgres", provider: "container", containerized: true},
		{rType: "postgres", provider: "aws", rds: true, custom: true},
		{rType: "mysql", provider: "aws", rds: true, custom: true},
		{rType: "mariadb", provider: "aws", rds: true, custom: true},
		{rType: "redis", containerized: true},
		{rType: "redis", provider: "aws", elasticache: true, custom: true},
		{rType: "memcached", provider: "aws", elasticache: true, custom: true},
		{rType: "postgis", containerized: true},
		{rType: "postgis", provider: "aws", custom: true},
		{rType: "rds-postgres", rds: true, custom: true},
		{rType: "rds-postgres", provider: "aws", rds: true, custom: true},
		{rType: "elasticache-redis", elasticache: true, custom: true},
		{rType: "elasticache-redis", provider: "aws", elasticache: true, custom: true},
		{rType: "webhook"},
	} {
		r := Resource{Name: "res1", Type: tc.rType, Provider: tc.provider}

		require.Equal(t, tc.rds, r.IsRds(), "IsRds for %s/%s", tc.rType, tc.provider)
		require.Equal(t, tc.elasticache, r.IsElastiCache(), "IsElastiCache for %s/%s", tc.rType, tc.provider)
		require.Equal(t, tc.containerized, r.IsContainerizedResource(), "IsContainerizedResource for %s/%s", tc.rType, tc.provider)
		require.Equal(t, tc.custom, r.IsCustomManagedResource(), "IsCustomManagedResource for %s/%s", tc.rType, tc.provider)
	}
}

func TestResourceProviderParse(t *testing.T) {
	m, err := Load([]byte("resources:\n  db:\n    type: postgres\n    provider: aws\n  cache:\n    type: redis\n"), nil)
	require.NoError(t, err)
	require.Len(t, m.Resources, 2)

	db, err := m.Resource("db")
	require.NoError(t, err)
	require.Equal(t, "aws", db.Provider)
	require.True(t, db.IsRds())

	cache, err := m.Resource("cache")
	require.NoError(t, err)
	require.Equal(t, "", cache.Provider)
	require.True(t, cache.IsContainerizedResource())
}

func TestValidateResourceProvider(t *testing.T) {
	for _, provider := range []string{"", "aws", "container"} {
		m := &Manifest{Resources: Resources{{Name: "db", Type: "postgres", Provider: provider}}}
		require.Empty(t, m.validateResources(), "provider %q", provider)
	}

	m := &Manifest{Resources: Resources{{Name: "db", Type: "postgres", Provider: "gcp"}}}
	errs := m.validateResources()
	require.Len(t, errs, 1)
	require.EqualError(t, errs[0], `resource "db" has unknown provider "gcp"`)
}
