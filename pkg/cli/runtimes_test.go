package cli_test

import (
	"fmt"
	"testing"

	"github.com/convox/convox/pkg/cli"
	mocksdk "github.com/convox/convox/pkg/mock/sdk"
	"github.com/convox/convox/pkg/structs"
	"github.com/stretchr/testify/require"
)

func TestRuntimesSuccess(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("OrganizationRuntimes", "testorg").Return(structs.Runtimes{*fxRuntime()}, nil)

		res, err := testExecute(e, "runtimes testorg", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		res.RequireStderr(t, []string{""})
		res.RequireStdout(t, []string{
			"ID                                    TITLE",
			"b29266a2-0d25-4194-b375-a7ac722f82a5  533267189958",
		})
	})
}

func TestRuntimesError(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("OrganizationRuntimes", "fakeorg").Return(nil, fmt.Errorf("organization not found"))

		res, err := testExecute(e, "runtimes fakeorg", nil)
		require.NoError(t, err)
		require.Equal(t, 1, res.Code)
		res.RequireStderr(t, []string{"ERROR: organization not found"})
		res.RequireStdout(t, []string{""})
	})
}
