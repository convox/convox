package cli_test

import (
	"testing"
	"time"

	"github.com/convox/convox/pkg/cli"
	mocksdk "github.com/convox/convox/pkg/mock/sdk"
	"github.com/convox/convox/pkg/options"
	"github.com/convox/convox/pkg/structs"
	"github.com/convox/stdcli"
	"github.com/stretchr/testify/require"
)

func TestRackUpdateDowngradeImdsRestricted(t *testing.T) {
	prev := cli.IsTerminalFn
	cli.IsTerminalFn = func(_ *stdcli.Context) bool { return false }
	t.Cleanup(func() { cli.IsTerminalFn = prev })

	system := func(version, hopLimit string) *structs.System {
		return &structs.System{
			Name:     "name",
			Provider: "aws",
			Status:   "running",
			Version:  version,
			Parameters: map[string]string{
				"imds_http_hop_limit": hopLimit,
				"imds_http_tokens":    "required",
			},
		}
	}

	for _, current := range []string{"3.26.2", "3.26.1", "3.26.1rc1"} {
		t.Run("refused from "+current, func(t *testing.T) {
			testClientWait(t, 50*time.Millisecond, func(e *cli.Engine, i *mocksdk.Interface) {
				i.On("SystemGet").Return(system(current, "1"), nil)

				res, err := testExecute(e, "rack update 3.26.0", nil)
				require.NoError(t, err)
				require.Equal(t, 1, res.Code)
				res.RequireStderr(t, []string{
					"ERROR: cannot downgrade to 3.26.0 while imds_http_hop_limit=1 and imds_http_tokens=required are set.",
					"  Set imds_http_hop_limit=3 first: convox rack params set imds_http_hop_limit=3",
				})
			})
		})
	}

	accepted := []struct {
		name     string
		current  string
		hopLimit string
		cmd      string
		version  string
		force    bool
	}{
		{"forced", "3.26.2", "1", "rack update 3.26.0 --force", "3.26.0", true},
		{"hop limit raised", "3.26.2", "3", "rack update 3.26.0", "3.26.0", false},
		{"target has the fix", "3.26.2", "1", "rack update 3.26.1", "3.26.1", false},
		{"target does not parse", "3.26.2", "1", "rack update dev1", "dev1", false},
		{"release candidate to its release", "3.26.1rc1", "1", "rack update 3.26.1", "3.26.1", false},
		{"both versions below the fix", "3.25.13", "1", "rack update 3.25.12", "3.25.12", false},
	}

	for _, tt := range accepted {
		t.Run(tt.name, func(t *testing.T) {
			testClientWait(t, 50*time.Millisecond, func(e *cli.Engine, i *mocksdk.Interface) {
				i.On("SystemGet").Return(system(tt.current, tt.hopLimit), nil)
				i.On("SystemUpdate", structs.SystemUpdateOptions{
					Version: options.String(tt.version),
					Force:   options.Bool(tt.force),
				}).Return(nil)

				res, err := testExecute(e, tt.cmd, nil)
				require.NoError(t, err)
				require.Equal(t, 0, res.Code)
			})
		})
	}
}
