package cli_test

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"testing"

	"github.com/convox/convox/pkg/cli"
	mocksdk "github.com/convox/convox/pkg/mock/sdk"
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
			"server: 21000101000000",
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
