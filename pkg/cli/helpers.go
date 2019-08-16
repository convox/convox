package cli

import (
	"os"
	"path/filepath"

	"github.com/convox/convox/pkg/common"
	"github.com/convox/stdcli"
)

func app(c *stdcli.Context) string {
	wd, err := os.Getwd()
	if err != nil {
		panic(err)
	}

	return common.CoalesceString(c.String("app"), filepath.Base(wd))
}

func rack(c *stdcli.Context) string {
	return common.CoalesceString(c.String("rack"), os.Getenv("CONVOX_RACK"))
}
