package cli_test

import (
	"fmt"
	"testing"

	"github.com/convox/convox/pkg/cli"
	mocksdk "github.com/convox/convox/pkg/mock/sdk"
	"github.com/convox/convox/pkg/structs"
	"github.com/stretchr/testify/require"
)

func TestInstances(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("InstanceList").Return(structs.Instances{*fxInstance(), *fxInstance()}, nil)

		res, err := testExecute(e, "instances", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		res.RequireStderr(t, []string{""})
		res.RequireStdout(t, []string{
			"ID         STATUS  STARTED     PS  CPU     MEM     PUBLIC  PRIVATE",
			"instance1  status  2 days ago  3   42.30%  71.80%  public  private",
			"instance1  status  2 days ago  3   42.30%  71.80%  public  private",
		})
	})
}

func TestInstancesError(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("InstanceList").Return(nil, fmt.Errorf("err1"))

		res, err := testExecute(e, "instances", nil)
		require.NoError(t, err)
		require.Equal(t, 1, res.Code)
		res.RequireStderr(t, []string{"ERROR: err1"})
		res.RequireStdout(t, []string{""})
	})
}

func TestInstancesTerminate(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("InstanceTerminate", "instance1").Return(nil)

		res, err := testExecute(e, "instances terminate instance1", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		res.RequireStderr(t, []string{""})
		res.RequireStdout(t, []string{"Terminating instance... OK"})
	})
}

func TestInstancesTerminateError(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("InstanceTerminate", "instance1").Return(fmt.Errorf("err1"))

		res, err := testExecute(e, "instances terminate instance1", nil)
		require.NoError(t, err)
		require.Equal(t, 1, res.Code)
		res.RequireStderr(t, []string{"ERROR: err1"})
		res.RequireStdout(t, []string{"Terminating instance... "})
	})
}
