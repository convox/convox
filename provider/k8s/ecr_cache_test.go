package k8s

import (
	"testing"
)

func TestResourceImage(t *testing.T) {
	prefix := "123456789012.dkr.ecr.us-east-1.amazonaws.com/docker-hub"

	tests := []struct {
		name         string
		prefix       string
		resourceType string
		version      string
		want         string
	}{
		{name: "redis", prefix: prefix, resourceType: "redis", version: "8", want: prefix + "/library/redis:8"},
		{name: "redis custom version", prefix: prefix, resourceType: "redis", version: "7.0", want: prefix + "/library/redis:7.0"},
		{name: "postgres", prefix: prefix, resourceType: "postgres", version: "16-trixie", want: prefix + "/library/postgres:16-trixie"},
		{name: "mysql", prefix: prefix, resourceType: "mysql", version: "8.4", want: prefix + "/library/mysql:8.4"},
		{name: "mariadb", prefix: prefix, resourceType: "mariadb", version: "11.4", want: prefix + "/library/mariadb:11.4"},
		{name: "memcached", prefix: prefix, resourceType: "memcached", version: "1.6", want: prefix + "/library/memcached:1.6"},
		{name: "postgis non-library image", prefix: prefix, resourceType: "postgis", version: "16-3.6", want: prefix + "/postgis/postgis:16-3.6"},
		{name: "postgis custom version", prefix: prefix, resourceType: "postgis", version: "15-3.3", want: prefix + "/postgis/postgis:15-3.3"},
		{name: "unknown resource type", prefix: prefix, resourceType: "unknown", version: "1", want: ""},
		{name: "trailing slash on prefix", prefix: prefix + "/", resourceType: "redis", version: "8", want: prefix + "/library/redis:8"},
		{name: "redis no prefix", resourceType: "redis", version: "8", want: "redis:8"},
		{name: "postgres no prefix", resourceType: "postgres", version: "10.5", want: "postgres:10.5"},
		{name: "mysql no prefix", resourceType: "mysql", version: "5.7.23", want: "mysql:5.7.23"},
		{name: "mariadb no prefix", resourceType: "mariadb", version: "11.4", want: "mariadb:11.4"},
		{name: "memcached no prefix", resourceType: "memcached", version: "1.6", want: "memcached:1.6"},
		{name: "postgis no prefix", resourceType: "postgis", version: "16-3.6", want: "postgis/postgis:16-3.6"},
		{name: "unknown no prefix", resourceType: "unknown", version: "1", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resourceImage(tt.prefix, tt.resourceType, tt.version)
			if got != tt.want {
				t.Errorf("resourceImage() = %q, want %q", got, tt.want)
			}
		})
	}
}
