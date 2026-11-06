package common

import (
	"reflect"
	"testing"
)

func TestBuildArchs(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"", []string{}},
		{"amd64", []string{"amd64"}},
		{"arm64", []string{"arm64"}},
		{"amd64,arm64", []string{"amd64", "arm64"}},
		{"arm64,amd64", []string{"amd64", "arm64"}},
		{" arm64 , amd64 ", []string{"amd64", "arm64"}},
		{"amd64,amd64", []string{"amd64"}},
		{"amd64,,arm64", []string{"amd64", "arm64"}},
		{"bogus", []string{}},
		{"amd64,bogus,arm64", []string{"amd64", "arm64"}},
	}

	for _, c := range cases {
		if got := BuildArchs(c.in); !reflect.DeepEqual(got, c.want) {
			t.Errorf("BuildArchs(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}
