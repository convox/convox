package cli_test

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"testing"

	"github.com/convox/convox/pkg/cli"
	mocksdk "github.com/convox/convox/pkg/mock/sdk"
	mockstdcli "github.com/convox/convox/pkg/mock/stdcli"
	"github.com/stretchr/testify/require"
)

func TestVersion(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		require.NoError(t, testLocalRack(e, "dev1", "local", "https://host1"))

		err := ioutil.WriteFile(filepath.Join(e.Settings, "host"), []byte("host1"), 0644)
		require.NoError(t, err)

		i.On("SystemGet").Return(fxSystem(), nil)

		res, err := testExecute(e, "version", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		res.RequireStderr(t, []string{""})
		res.RequireStdout(t, []string{
			"client: test",
			"server: 21000101000000 (host1)",
		})
	})
}

func TestVersionError(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		require.NoError(t, testLocalRack(e, "dev1", "local", "https://host1"))

		err := ioutil.WriteFile(filepath.Join(e.Settings, "host"), []byte("host1"), 0644)
		require.NoError(t, err)

		i.On("SystemGet").Return(nil, fmt.Errorf("err1"))

		res, err := testExecute(e, "version", nil)
		require.NoError(t, err)
		require.Equal(t, 1, res.Code)
		res.RequireStderr(t, []string{"ERROR: err1"})
		res.RequireStdout(t, []string{
			"client: test",
		})
	})
}

func TestVersionNoSystem(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		me := &mockstdcli.Executor{}
		me.On("Execute", "kubectl", "get", "ns", "--selector=system=convox,type=rack", "--output=name").Return([]byte(""), nil)
		e.Executor = me

		res, err := testExecute(e, "version", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		res.RequireStderr(t, []string{""})
		res.RequireStdout(t, []string{
			"client: test",
			"server: none",
		})
	})
}

func TestVersionNoSystemMultipleLocal(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		me := &mockstdcli.Executor{}
		me.On("Execute", "kubectl", "get", "ns", "--selector=system=convox,type=rack", "--output=name").Return([]byte("namespace/dev\nnamespace/dev2\n"), nil)
		me.On("Execute", "kubectl", "get", "namespace/dev", "-o", "jsonpath={.metadata.labels.rack}").Return([]byte("dev\n"), nil)
		me.On("Execute", "kubectl", "get", "namespace/dev2", "-o", "jsonpath={.metadata.labels.rack}").Return([]byte("dev2\n"), nil)
		e.Executor = me

		res, err := testExecute(e, "version", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		res.RequireStderr(t, []string{""})
		res.RequireStdout(t, []string{
			"client: test",
			"server: none",
		})
	})
}

func TestVersionNoSystemSingleLocal(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		require.NoError(t, testLocalRack(e, "dev1", "local", "https://api.dev.convox"))

		i.On("SystemGet").Return(fxSystemLocal(), nil)

		res, err := testExecute(e, "version", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		res.RequireStderr(t, []string{""})
		res.RequireStdout(t, []string{
			"client: test",
			"server: dev1 (api.dev.convox)",
		})
	})
}
