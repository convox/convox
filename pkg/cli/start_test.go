package cli_test

import (
	"fmt"
	"testing"

	"github.com/convox/convox/pkg/cli"
	mocksdk "github.com/convox/convox/pkg/mock/sdk"
	mockstart "github.com/convox/convox/pkg/mock/start"
	mockstdcli "github.com/convox/convox/pkg/mock/stdcli"
	"github.com/convox/convox/pkg/start"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestStart2(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		me := &mockstdcli.Executor{}
		me.On("Execute", "kubectl", "get", "ns", "--selector=system=convox,type=rack", "--output=name").Return([]byte("namespace/dev\n"), nil)
		e.Executor = me

		ms := &mockstart.Interface{}
		cli.Starter = ms

		opts := start.Options2{
			App:      "app1",
			Build:    true,
			Cache:    true,
			Provider: i,
			Sync:     true,
		}

		ms.On("Start2", mock.Anything, mock.Anything, opts).Return(nil)

		res, err := testExecute(e, "start -g 2 -a app1", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		res.RequireStderr(t, []string{""})
		res.RequireStdout(t, []string{""})
	})
}

func TestStart2Error(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		me := &mockstdcli.Executor{}
		me.On("Execute", "kubectl", "get", "ns", "--selector=system=convox,type=rack", "--output=name").Return([]byte("namespace/dev\n"), nil)
		e.Executor = me

		ms := &mockstart.Interface{}
		cli.Starter = ms

		opts := start.Options2{
			App:      "app1",
			Build:    true,
			Cache:    true,
			Provider: i,
			Sync:     true,
		}

		ms.On("Start2", mock.Anything, mock.Anything, opts).Return(fmt.Errorf("err1"))

		res, err := testExecute(e, "start -g 2 -a app1", nil)
		require.NoError(t, err)
		require.Equal(t, 1, res.Code)
		res.RequireStderr(t, []string{"ERROR: err1"})
		res.RequireStdout(t, []string{""})
	})
}

func TestStart2Options(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		me := &mockstdcli.Executor{}
		me.On("Execute", "kubectl", "get", "ns", "--selector=system=convox,type=rack", "--output=name").Return([]byte("namespace/dev\n"), nil)
		e.Executor = me

		ms := &mockstart.Interface{}
		cli.Starter = ms

		opts := start.Options2{
			App:      "app1",
			Build:    false,
			Cache:    false,
			Manifest: "manifest1",
			Provider: i,
			Services: []string{"service1", "service2"},
			Sync:     false,
		}

		ms.On("Start2", mock.Anything, mock.Anything, opts).Return(nil)

		res, err := testExecute(e, "start -g 2 -a app1 -m manifest1 --no-build --no-cache --no-sync service1 service2", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		res.RequireStderr(t, []string{""})
		res.RequireStdout(t, []string{""})
	})
}

// func TestStart2Remote(t *testing.T) {
// 	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
// 		me := &mockstdcli.Executor{}
// 		me.On("Execute", "kubectl", "get", "ns", "--selector=system=convox,type=rack", "--output=name").Return([]byte("namespace/dev"), nil)
// 		e.Executor = me

// 		ms := &mockstart.Interface{}
// 		cli.Starter = ms

// 		ms.On("Start2", mock.Anything, mock.Anything, mock.Anything).Return(nil).Run(func(args mock.Arguments) {
// 			opts := args.Get(2).(start.Options2)
// 			require.Equal(t, "app1", opts.App)
// 			require.Equal(t, true, opts.Build)
// 			require.Equal(t, true, opts.Cache)
// 			require.Equal(t, true, opts.Sync)
// 			fmt.Printf("opts.Provider: %+v\n", opts.Provider)
// 			p := opts.Provider.(*sdk.Client)
// 			require.Equal(t, "https", p.Client.Endpoint.Scheme)
// 			require.Equal(t, "rack.dev", p.Client.Endpoint.Host)
// 		})

// 		res, err := testExecute(e, "start -g 2 -a app1", nil)
// 		require.NoError(t, err)
// 		require.Equal(t, 0, res.Code)
// 		res.RequireStderr(t, []string{""})
// 		res.RequireStdout(t, []string{""})
// 	})
// }

// func TestStart2RemoteMultiple(t *testing.T) {
// 	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
// 		me := &mockstdcli.Executor{}
// 		me.On("Execute", "kubectl", "get", "ns", "--selector=system=convox,type=rack", "--output=name").Return([]byte("namespace/dev\nnamespace/dev2\n"), nil)
// 		e.Executor = me

// 		ms := &mockstart.Interface{}
// 		cli.Starter = ms

// 		opts := start.Options2{
// 			App:   "app1",
// 			Build: true,
// 			Cache: true,
// 			Sync:  true,
// 		}

// 		ms.On("Start2", mock.Anything, opts).Return(nil).Run(func(args mock.Arguments) {
// 			s := args.Get(0).(*sdk.Client)
// 			require.Equal(t, "https", s.Client.Endpoint.Scheme)
// 			require.Equal(t, "rack.classic", s.Client.Endpoint.Host)
// 		})

// 		res, err := testExecute(e, "start -g 2 -a app1", nil)
// 		require.NoError(t, err)
// 		require.Equal(t, 1, res.Code)
// 		res.RequireStderr(t, []string{"ERROR: multiple local racks detected, use `convox switch` to select one"})
// 		res.RequireStdout(t, []string{""})
// 	})
// }
