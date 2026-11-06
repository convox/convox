package structs_test

import (
	"testing"

	"github.com/convox/convox/pkg/structs"
	"github.com/stretchr/testify/require"
)

func TestReservedAppName(t *testing.T) {
	for _, name := range []string{"rack", "system"} {
		require.True(t, structs.ReservedAppName(name))
	}

	for _, name := range []string{"app1", "systemd", "myrack", "Rack", "System", ""} {
		require.False(t, structs.ReservedAppName(name))
	}
}
