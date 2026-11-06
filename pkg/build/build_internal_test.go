package build

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuildEnvs(t *testing.T) {
	tests := []struct {
		Name string
		Tid  string
		Env  map[string]string
	}{
		{Name: "SingleTenant", Tid: "", Env: map[string]string{"BUILD_APP": "app1", "BUILD_AUTH": "{}", "BUILD_PUSH": "registry.test.com", "FOO": "bar", "RACK_URL": "https://api.rack1.svc.cluster.local:5443"}},
		{Name: "Tenant", Tid: "t1", Env: map[string]string{"BUILD_APP": "app1", "FOO": "bar"}},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			t.Setenv("BUILD_APP", "app1")
			t.Setenv("BUILD_AUTH", "{}")
			t.Setenv("BUILD_PUSH", "registry.test.com")
			t.Setenv("RACK_URL", "https://api.rack1.svc.cluster.local:5443")
			t.Setenv("CONVOX_TID", test.Tid)

			bb := &Build{Options: Options{BuildArgs: []string{"RACK_URL", "BUILD_APP", "BUILD_AUTH", "BUILD_PUSH", "FOO=bar"}}}

			env, err := bb.buildEnvs()
			require.NoError(t, err)
			require.Equal(t, test.Env, env)
		})
	}
}
