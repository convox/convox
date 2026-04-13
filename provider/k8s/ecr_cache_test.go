package k8s

import (
	"testing"
)

func TestEcrCachedResourceImage(t *testing.T) {
	prefix := "123456789012.dkr.ecr.us-east-1.amazonaws.com/docker-hub"

	tests := []struct {
		name         string
		resourceType string
		options      map[string]string
		want         string
	}{
		{
			name:         "redis default version",
			resourceType: "redis",
			options:      map[string]string{},
			want:         prefix + "/library/redis:4.0.10",
		},
		{
			name:         "redis custom version",
			resourceType: "redis",
			options:      map[string]string{"version": "7.0"},
			want:         prefix + "/library/redis:7.0",
		},
		{
			name:         "postgres default version",
			resourceType: "postgres",
			options:      map[string]string{},
			want:         prefix + "/library/postgres:10.5",
		},
		{
			name:         "mysql default version",
			resourceType: "mysql",
			options:      map[string]string{},
			want:         prefix + "/library/mysql:5.7.23",
		},
		{
			name:         "mariadb default version",
			resourceType: "mariadb",
			options:      map[string]string{},
			want:         prefix + "/library/mariadb:10.6.0",
		},
		{
			name:         "memcached default version",
			resourceType: "memcached",
			options:      map[string]string{},
			want:         prefix + "/library/memcached:1.4.34",
		},
		{
			name:         "postgis non-library image",
			resourceType: "postgis",
			options:      map[string]string{},
			want:         prefix + "/postgis/postgis:10-3.2",
		},
		{
			name:         "postgis custom version",
			resourceType: "postgis",
			options:      map[string]string{"version": "15-3.3"},
			want:         prefix + "/postgis/postgis:15-3.3",
		},
		{
			name:         "unknown resource type",
			resourceType: "unknown",
			options:      map[string]string{},
			want:         "",
		},
		{
			name:         "trailing slash on prefix",
			resourceType: "redis",
			options:      map[string]string{},
			want:         prefix + "/library/redis:4.0.10",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := prefix
			if tt.name == "trailing slash on prefix" {
				p = prefix + "/"
			}
			got := ecrCachedResourceImage(p, tt.resourceType, tt.options)
			if got != tt.want {
				t.Errorf("ecrCachedResourceImage() = %q, want %q", got, tt.want)
			}
		})
	}
}
