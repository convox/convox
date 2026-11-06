package cli

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/convox/convox/pkg/common"
	"github.com/convox/convox/pkg/structs"
	"github.com/convox/convox/sdk"
	"github.com/convox/stdcli"
)

func init() {
	register("cp", "copy files", Cp, stdcli.CommandOptions{
		Flags:    append([]stdcli.Flag{flagApp, flagRack}, stdcli.OptionFlags(structs.FileTransterOptions{})...),
		Usage:    "<[pid:]src> <[pid:]dst>",
		Validate: stdcli.Args(2),
	}, WithCloud())
}

func Cp(rack sdk.Interface, c *stdcli.Context) error {
	src := c.Arg(0)
	dst := c.Arg(1)

	var opts structs.FileTransterOptions
	if err := c.Options(&opts); err != nil {
		return err
	}

	r, err := cpSource(rack, c, src)
	if err != nil {
		return err
	}

	if err := cpDestination(rack, c, r, dst, opts); err != nil {
		return err
	}

	return nil
}

func cpDestination(rack sdk.Interface, c *stdcli.Context, r io.Reader, dst string, opts structs.FileTransterOptions) error {
	parts := strings.SplitN(dst, ":", 2)

	switch len(parts) {
	case 1:
		abs, err := filepath.Abs(parts[0])
		if err != nil {
			return err
		}

		rr, err := common.RebaseArchive(r, "/base", abs)
		if err != nil {
			return err
		}

		return common.Unarchive(rr, "/")
	case 2:
		if !strings.HasPrefix(parts[1], "/") {
			return fmt.Errorf("must specify absolute paths for processes")
		}

		rr, err := common.RebaseArchive(r, "/base", parts[1])
		if err != nil {
			return err
		}

		return rack.FilesUpload(app(c), parts[0], rr, opts)
	default:
		return fmt.Errorf("unknown destination: %s", dst)
	}
}

func cpSource(rack sdk.Interface, c *stdcli.Context, src string) (io.Reader, error) {
	parts := strings.SplitN(src, ":", 2)

	switch len(parts) {
	case 1:
		abs, err := filepath.Abs(parts[0])
		if err != nil {
			return nil, err
		}

		r, err := common.Archive(abs)
		if err != nil {
			return nil, err
		}

		return common.RebaseArchive(r, abs, "/base")
	case 2:
		if !strings.HasPrefix(parts[1], "/") {
			return nil, fmt.Errorf("must specify absolute paths for processes")
		}

		r, err := rack.FilesDownload(app(c), parts[0], parts[1])
		if err != nil {
			return nil, err
		}

		return common.RebaseArchive(r, parts[1], "/base")
	default:
		return nil, fmt.Errorf("unknown source: %s", src)
	}
}
