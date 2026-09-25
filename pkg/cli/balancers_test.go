package cli_test

import (
	"fmt"
	"testing"

	"github.com/convox/convox/pkg/cli"
	mocksdk "github.com/convox/convox/pkg/mock/sdk"
	"github.com/convox/convox/pkg/structs"
	"github.com/convox/stdcli"
	"github.com/stretchr/testify/require"
)

func fxBalancers() structs.Balancers {
	return structs.Balancers{
		{Name: "balancer1", Service: "service1", Endpoint: "balancer1.example.org"},
		{Name: "balancer2", Service: "service2"},
	}
}

func TestBalancers(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("BalancerList", "app1").Return(fxBalancers(), nil)

		res, err := testExecute(e, "balancers -a app1", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		res.RequireStderr(t, []string{""})
		res.RequireStdout(t, []string{
			"BALANCER   SERVICE   ENDPOINT",
			"balancer1  service1  balancer1.example.org",
			"balancer2  service2  ",
		})
	})
}

func TestBalancersTerminal(t *testing.T) {
	prev := cli.IsTerminalFn
	cli.IsTerminalFn = func(_ *stdcli.Context) bool { return true }
	t.Cleanup(func() { cli.IsTerminalFn = prev })

	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		bs := fxBalancers()
		i.On("BalancerList", "app1").Return(bs, nil)

		res, err := testExecute(e, "balancers -a app1", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		res.RequireStderr(t, []string{""})
		res.RequireStdout(t, []string{
			"BALANCER   SERVICE   ENDPOINT",
			"balancer1  service1  balancer1.example.org",
			"balancer2  service2  (pending)",
		})
		require.Equal(t, "", bs[1].Endpoint)
	})
}

func TestBalancersError(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("BalancerList", "app1").Return(nil, fmt.Errorf("err1"))

		res, err := testExecute(e, "balancers -a app1", nil)
		require.NoError(t, err)
		require.Equal(t, 1, res.Code)
		res.RequireStderr(t, []string{"ERROR: err1"})
		res.RequireStdout(t, []string{""})
	})
}
