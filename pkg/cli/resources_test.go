package cli_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/convox/convox/pkg/cli"
	mocksdk "github.com/convox/convox/pkg/mock/sdk"
	"github.com/convox/convox/pkg/options"
	"github.com/convox/convox/pkg/structs"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestResources(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("ResourceList", "app1").Return(structs.Resources{*fxResource(), *fxResource()}, nil)

		res, err := testExecute(e, "resources -a app1", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		res.RequireStderr(t, []string{""})
		res.RequireStdout(t, []string{
			"NAME       TYPE  URL",
			"resource1  type  https://example.org/path",
			"resource1  type  https://example.org/path",
		})
	})
}

func TestResourcesError(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("ResourceList", "app1").Return(nil, fmt.Errorf("err1"))

		res, err := testExecute(e, "resources -a app1", nil)
		require.NoError(t, err)
		require.Equal(t, 1, res.Code)
		res.RequireStderr(t, []string{"ERROR: err1"})
		res.RequireStdout(t, []string{""})
	})
}

func TestResourcesExport(t *testing.T) {
	dump := bytes.Repeat([]byte("INSERT INTO t VALUES (1);\n"), 6000)
	early := append([]byte("ERROR: could not export: in data\n"), bytes.Repeat([]byte("x"), 70*1024)...)

	tests := []struct {
		name   string
		data   []byte
		reader func(io.Reader) io.Reader
		file   bool
	}{
		{"file", []byte("-- dump\n"), iotest.DataErrReader, true},
		{"stdout", dump, func(r io.Reader) io.Reader { return r }, false},
		{"stdout one byte reads", dump, iotest.OneByteReader, false},
		{"marker before window", early, func(r io.Reader) io.Reader { return r }, false},
		{"marker before window one byte reads", early, iotest.OneByteReader, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
				i.On("ResourceExport", "app1", "resource1").Return(io.NopCloser(tt.reader(bytes.NewReader(tt.data))), nil)

				if tt.file {
					file := filepath.Join(t.TempDir(), "out.sql")

					res, err := testExecute(e, fmt.Sprintf("resources export resource1 -a app1 -f %s", file), nil)
					require.NoError(t, err)
					require.Equal(t, 0, res.Code)
					res.RequireStderr(t, []string{""})
					res.RequireStdout(t, []string{"Exporting resource data... OK"})

					data, err := os.ReadFile(file)
					require.NoError(t, err)
					require.Equal(t, tt.data, data)
					return
				}

				res, err := testExecute(e, "resources export resource1 -a app1", nil)
				require.NoError(t, err)
				require.Equal(t, 0, res.Code)
				res.RequireStderr(t, []string{"Exporting resource data... OK"})
				require.Equal(t, string(tt.data), res.Stdout)
			})
		})
	}
}

func TestResourcesExportError(t *testing.T) {
	tests := []struct {
		name   string
		reader io.Reader
		target string
		stderr []string
	}{
		{
			name:   "error after marker",
			reader: strings.NewReader("ERROR: could not export: exit status 2: line1\nline2\n"),
			target: "file",
			stderr: []string{"ERROR: could not export: exit status 2: line1", "line2"},
		},
		{
			name:   "tool lines before marker",
			reader: strings.NewReader("mysqldump: [Warning] x\nmysqldump: Got error: 2026: y\nERROR: could not export: exit status 2\n"),
			stderr: []string{"Exporting resource data... ERROR: could not export: exit status 2: mysqldump: [Warning] x", "mysqldump: Got error: 2026: y"},
		},
		{
			name:   "pg_dump error and detail",
			reader: strings.NewReader("pg_dump: error: e\npg_dump: detail: d\nERROR: could not export: exit status 1\n"),
			stderr: []string{"Exporting resource data... ERROR: could not export: exit status 1: pg_dump: error: e", "pg_dump: detail: d"},
		},
		{
			name:   "copy rows between tool lines and marker",
			reader: strings.NewReader("COPY public.t (a, b) FROM stdin;\n1\tx\n2pg_dump: error: e\npg_dump: detail: Command was: COPY public.t (a, b) TO stdout;\n\tsecret\n3\ty\nERROR: could not export: exit status 1\n"),
			stderr: []string{"Exporting resource data... ERROR: could not export: exit status 1: pg_dump: detail: Command was: COPY public.t (a, b) TO stdout;"},
		},
		{
			name:   "sql between tool lines and marker",
			reader: strings.NewReader("mysqldump: [Warning] x\nmysqldump: Got error: 1044: y\n-- MySQL dump\n/*!40101 SET x */;\nERROR: could not export: exit status 2\n"),
			stderr: []string{"Exporting resource data... ERROR: could not export: exit status 2: mysqldump: [Warning] x", "mysqldump: Got error: 1044: y"},
		},
		{
			name:   "read error",
			reader: io.MultiReader(strings.NewReader("-- partial"), iotest.ErrReader(fmt.Errorf("read failed"))),
			target: "file",
			stderr: []string{"ERROR: read failed"},
		},
		{
			name:   "symlink kept",
			reader: strings.NewReader("ERROR: could not export: exit status 2\n"),
			target: "link",
			stderr: []string{"ERROR: could not export: exit status 2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
				i.On("ResourceExport", "app1", "resource1").Return(io.NopCloser(tt.reader), nil)

				if tt.target == "" {
					res, err := testExecute(e, "resources export resource1 -a app1", nil)
					require.NoError(t, err)
					require.Equal(t, 1, res.Code)
					res.RequireStderr(t, tt.stderr)
					require.Equal(t, "", res.Stdout)
					return
				}

				dir := t.TempDir()
				file := filepath.Join(dir, "out.sql")

				if tt.target == "link" {
					require.NoError(t, os.WriteFile(filepath.Join(dir, "target.sql"), nil, 0600))
					require.NoError(t, os.Symlink("target.sql", file))
				}

				res, err := testExecute(e, fmt.Sprintf("resources export resource1 -a app1 -f %s", file), nil)
				require.NoError(t, err)
				require.Equal(t, 1, res.Code)
				res.RequireStderr(t, tt.stderr)
				res.RequireStdout(t, []string{"Exporting resource data... "})

				_, err = os.Lstat(file)
				if tt.target == "link" {
					require.NoError(t, err)
				} else {
					require.True(t, os.IsNotExist(err))
				}
			})
		})
	}
}

func TestResourcesExportErrorMidDump(t *testing.T) {
	dump := bytes.Repeat([]byte("INSERT INTO t VALUES (1);\n"), 6000)
	dump = dump[:len(dump)-5]
	data := append(append([]byte{}, dump...), "ERROR: could not export: signal: killed\n"...)

	readers := map[string]func(io.Reader) io.Reader{
		"plain":    func(r io.Reader) io.Reader { return r },
		"one byte": iotest.OneByteReader,
	}

	for name, reader := range readers {
		t.Run(name, func(t *testing.T) {
			testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
				i.On("ResourceExport", "app1", "resource1").Return(io.NopCloser(reader(bytes.NewReader(data))), nil)

				res, err := testExecute(e, "resources export resource1 -a app1", nil)
				require.NoError(t, err)
				require.Equal(t, 1, res.Code)
				res.RequireStderr(t, []string{"Exporting resource data... ERROR: could not export: signal: killed"})
				require.NotEmpty(t, res.Stdout)
				require.True(t, strings.HasPrefix(string(dump), res.Stdout))
			})
		})
	}
}

func TestResourcesInfo(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("ResourceGet", "app1", "resource1").Return(fxResource(), nil)

		res, err := testExecute(e, "resources info resource1 -a app1", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		res.RequireStderr(t, []string{""})
		res.RequireStdout(t, []string{
			"Name  resource1",
			"Type  type",
			"URL   https://example.org/path",
		})
	})
}

func TestResourcesInfoError(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("ResourceGet", "app1", "resource1").Return(nil, fmt.Errorf("err1"))

		res, err := testExecute(e, "resources info resource1 -a app1", nil)
		require.NoError(t, err)
		require.Equal(t, 1, res.Code)
		res.RequireStderr(t, []string{"ERROR: err1"})
		res.RequireStdout(t, []string{""})
	})
}

func TestResourcesProxy(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		i.On("WithContext", ctx).Return(i)
		i.On("ResourceGet", "app1", "resource1").Return(fxResource(), nil)
		i.On("Proxy", "example.org", 443, mock.Anything, structs.ProxyOptions{TLS: options.Bool(false)}).Return(nil).Run(func(args mock.Arguments) {
			buf := make([]byte, 2)
			rwc := args.Get(2).(io.ReadWriteCloser)
			n, err := rwc.Read(buf)
			require.NoError(t, err)
			require.Equal(t, 2, n)
			require.Equal(t, "in", string(buf))
			n, err = rwc.Write([]byte("out"))
			require.NoError(t, err)
			require.Equal(t, 3, n)
			rwc.Close()
		})

		port := freePort(t)

		ch := make(chan *result)

		go func() {
			res, _ := testExecuteContext(ctx, e, fmt.Sprintf("resources proxy resource1 -a app1 -p %d", port), nil)
			ch <- res
		}()

		cn := dialEventually(t, port)

		cn.Write([]byte("in"))

		data, err := io.ReadAll(cn)
		require.NoError(t, err)
		require.Equal(t, "out", string(data))

		cancel()

		res := <-ch

		require.NotNil(t, res)
		require.Equal(t, 0, res.Code)
		res.RequireStderr(t, []string{""})
		res.RequireStdout(t, []string{
			fmt.Sprintf("proxying localhost:%d to example.org:443", port),
			fmt.Sprintf("connect: %d", port),
		})
	})
}

func TestResourcesUrl(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("ResourceGet", "app1", "resource1").Return(fxResource(), nil)

		res, err := testExecute(e, "resources url resource1 -a app1", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		res.RequireStderr(t, []string{""})
		res.RequireStdout(t, []string{"https://example.org/path"})
	})
}

func TestResourcesUrlError(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("ResourceGet", "app1", "resource1").Return(nil, fmt.Errorf("err1"))

		res, err := testExecute(e, "resources url resource1 -a app1", nil)
		require.NoError(t, err)
		require.Equal(t, 1, res.Code)
		res.RequireStderr(t, []string{"ERROR: err1"})
		res.RequireStdout(t, []string{""})
	})
}

func TestRackResources(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("SystemGet").Return(fxSystem(), nil)
		i.On("SystemResourceList").Return(structs.Resources{*fxResource(), *fxResource()}, nil)

		res, err := testExecute(e, "rack resources", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		res.RequireStderr(t, []string{""})
		res.RequireStdout(t, []string{
			"NAME       TYPE  STATUS",
			"resource1  type  running",
			"resource1  type  running",
		})
	})
}

func TestRackResourcesError(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("SystemGet").Return(fxSystem(), nil)
		i.On("SystemResourceList").Return(nil, fmt.Errorf("err1"))

		res, err := testExecute(e, "rack resources", nil)
		require.NoError(t, err)
		require.Equal(t, 1, res.Code)
		res.RequireStderr(t, []string{"ERROR: err1"})
		res.RequireStdout(t, []string{""})
	})
}

func TestRackResourcesCreate(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("SystemGet").Return(fxSystem(), nil)
		opts := structs.ResourceCreateOptions{Name: options.String("name1"), Parameters: map[string]string{"Foo": "bar", "Baz": "quux"}}
		i.On("SystemResourceCreate", "type1", opts).Return(fxResource(), nil)
		i.On("SystemResourceGet", "resource1").Return(fxResource(), nil).Once()
		i.On("SystemResourceGet", "resource1").Return(fxResourceUpdating(), nil).Twice()
		i.On("SystemResourceGet", "resource1").Return(fxResource(), nil)

		res, err := testExecute(e, "rack resources create type1 -n name1 Foo=bar Baz=quux", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		res.RequireStderr(t, []string{""})
		res.RequireStdout(t, []string{"Creating resource... OK, resource1"})
	})
}

func TestRackResourcesCreateError(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("SystemGet").Return(fxSystem(), nil)
		opts := structs.ResourceCreateOptions{Name: options.String("name1"), Parameters: map[string]string{"Foo": "bar", "Baz": "quux"}}
		i.On("SystemResourceCreate", "type1", opts).Return(nil, fmt.Errorf("err1"))

		res, err := testExecute(e, "rack resources create type1 -n name1 Foo=bar Baz=quux", nil)
		require.NoError(t, err)
		require.Equal(t, 1, res.Code)
		res.RequireStderr(t, []string{"ERROR: err1"})
		res.RequireStdout(t, []string{"Creating resource... "})
	})
}

func TestRackResourcesCreateClassic(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("SystemGet").Return(fxSystemClassic(), nil)
		opts := structs.ResourceCreateOptions{Name: options.String("name1"), Parameters: map[string]string{"Foo": "bar", "Baz": "quux"}}
		i.On("ResourceCreateClassic", "type1", opts).Return(fxResource(), nil)
		i.On("SystemResourceGetClassic", "resource1").Return(fxResource(), nil).Once()
		i.On("SystemResourceGetClassic", "resource1").Return(fxResourceUpdating(), nil).Twice()
		i.On("SystemResourceGetClassic", "resource1").Return(fxResource(), nil)

		res, err := testExecute(e, "rack resources create type1 -n name1 Foo=bar Baz=quux", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		res.RequireStderr(t, []string{""})
		res.RequireStdout(t, []string{"Creating resource... OK, resource1"})
	})
}

func TestRackResourcesDelete(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("SystemGet").Return(fxSystem(), nil)
		i.On("SystemResourceDelete", "resource1").Return(nil)
		i.On("SystemResourceGet", "resource1").Return(fxResource(), nil).Once()
		i.On("SystemResourceGet", "resource1").Return(fxResourceUpdating(), nil).Twice()
		i.On("SystemResourceGet", "resource1").Return(nil, fmt.Errorf("no such resource"))

		res, err := testExecute(e, "rack resources delete resource1", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		res.RequireStderr(t, []string{""})
		res.RequireStdout(t, []string{"Deleting resource... OK"})
	})
}

func TestRackResourcesDeleteError(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("SystemGet").Return(fxSystem(), nil)
		i.On("SystemResourceDelete", "resource1").Return(fmt.Errorf("err1"))

		res, err := testExecute(e, "rack resources delete resource1", nil)
		require.NoError(t, err)
		require.Equal(t, 1, res.Code)
		res.RequireStderr(t, []string{"ERROR: err1"})
		res.RequireStdout(t, []string{"Deleting resource... "})
	})
}

func TestRackResourcesInfo(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("SystemGet").Return(fxSystem(), nil)
		i.On("SystemResourceGet", "resource1").Return(fxResource(), nil)

		res, err := testExecute(e, "rack resources info resource1", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		res.RequireStderr(t, []string{""})
		res.RequireStdout(t, []string{
			"Name     resource1",
			"Type     type",
			"Status   running",
			"Options  Url=https://other.example.org/path",
			"         k1=v1",
			"         k2=v2",
			"URL      https://example.org/path",
			"Apps     app1, app1",
		})
	})
}

func TestRackResourcesInfoError(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("SystemGet").Return(fxSystem(), nil)
		i.On("SystemResourceGet", "resource1").Return(nil, fmt.Errorf("err1"))

		res, err := testExecute(e, "rack resources info resource1", nil)
		require.NoError(t, err)
		require.Equal(t, 1, res.Code)
		res.RequireStderr(t, []string{"ERROR: err1"})
		res.RequireStdout(t, []string{""})
	})
}

func TestRackResourcesLink(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("SystemGet").Return(fxSystem(), nil)
		i.On("SystemResourceLink", "resource1", "app1").Return(fxResource(), nil)
		i.On("SystemResourceGet", "resource1").Return(fxResource(), nil).Once()
		i.On("SystemResourceGet", "resource1").Return(fxResourceUpdating(), nil).Twice()
		i.On("SystemResourceGet", "resource1").Return(fxResource(), nil)

		res, err := testExecute(e, "rack resources link resource1 -a app1", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		res.RequireStderr(t, []string{""})
		res.RequireStdout(t, []string{"Linking to app1... OK"})
	})
}

func TestRackResourcesLinkError(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("SystemGet").Return(fxSystem(), nil)
		i.On("SystemResourceLink", "resource1", "app1").Return(nil, fmt.Errorf("err1"))

		res, err := testExecute(e, "rack resources link resource1 -a app1", nil)
		require.NoError(t, err)
		require.Equal(t, 1, res.Code)
		res.RequireStderr(t, []string{"ERROR: err1"})
		res.RequireStdout(t, []string{"Linking to app1... "})
	})
}

func TestRackResourcesOptions(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("SystemGet").Return(fxSystem(), nil)
		i.On("SystemResourceTypes").Return(structs.ResourceTypes{fxResourceType()}, nil)

		res, err := testExecute(e, "rack resources options type1", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		res.RequireStderr(t, []string{""})
		res.RequireStdout(t, []string{
			"NAME    DEFAULT  DESCRIPTION",
			"Param1  def1     desc1",
			"Param2  def2     desc2",
		})
	})
}

func TestRackResourcesOptionsError(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("SystemGet").Return(fxSystem(), nil)
		i.On("SystemResourceTypes").Return(nil, fmt.Errorf("err1"))

		res, err := testExecute(e, "rack resources options type1", nil)
		require.NoError(t, err)
		require.Equal(t, 1, res.Code)
		res.RequireStderr(t, []string{"ERROR: err1"})
		res.RequireStdout(t, []string{""})
	})
}

func TestRackResourcesProxy(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		i.On("WithContext", ctx).Return(i)
		i.On("SystemGet").Return(fxSystem(), nil)
		i.On("SystemResourceGet", "resource1").Return(fxResource(), nil)
		i.On("Proxy", "example.org", 443, mock.Anything, structs.ProxyOptions{TLS: options.Bool(false)}).Return(nil).Run(func(args mock.Arguments) {
			buf := make([]byte, 2)
			rwc := args.Get(2).(io.ReadWriteCloser)
			n, err := rwc.Read(buf)
			require.NoError(t, err)
			require.Equal(t, 2, n)
			require.Equal(t, "in", string(buf))
			n, err = rwc.Write([]byte("out"))
			require.NoError(t, err)
			require.Equal(t, 3, n)
			rwc.Close()
		})

		port := freePort(t)

		ch := make(chan *result)

		go func() {
			res, _ := testExecuteContext(ctx, e, fmt.Sprintf("rack resources proxy resource1 -p %d", port), nil)
			ch <- res
		}()

		cn := dialEventually(t, port)

		cn.Write([]byte("in"))

		data, err := io.ReadAll(cn)
		require.NoError(t, err)
		require.Equal(t, "out", string(data))

		cancel()

		res := <-ch

		require.NotNil(t, res)
		require.Equal(t, 0, res.Code)
		res.RequireStderr(t, []string{""})
		res.RequireStdout(t, []string{
			fmt.Sprintf("proxying localhost:%d to example.org:443", port),
			fmt.Sprintf("connect: %d", port),
		})
	})
}

func TestRackResourcesTypes(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("SystemGet").Return(fxSystem(), nil)
		i.On("SystemResourceTypes").Return(structs.ResourceTypes{fxResourceType(), fxResourceType()}, nil)

		res, err := testExecute(e, "rack resources types", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		res.RequireStderr(t, []string{""})
		res.RequireStdout(t, []string{
			"TYPE",
			"type1",
			"type1",
		})
	})
}

func TestRackResourcesTypesError(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("SystemGet").Return(fxSystem(), nil)
		i.On("SystemResourceTypes").Return(nil, fmt.Errorf("err1"))

		res, err := testExecute(e, "rack resources types", nil)
		require.NoError(t, err)
		require.Equal(t, 1, res.Code)
		res.RequireStderr(t, []string{"ERROR: err1"})
		res.RequireStdout(t, []string{""})
	})
}

func TestRackResourcesUnlink(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("SystemGet").Return(fxSystem(), nil)
		i.On("SystemResourceUnlink", "resource1", "app1").Return(fxResource(), nil)
		i.On("SystemResourceGet", "resource1").Return(fxResource(), nil).Once()
		i.On("SystemResourceGet", "resource1").Return(fxResourceUpdating(), nil).Twice()
		i.On("SystemResourceGet", "resource1").Return(fxResource(), nil)

		res, err := testExecute(e, "rack resources unlink resource1 -a app1", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		res.RequireStderr(t, []string{""})
		res.RequireStdout(t, []string{"Unlinking from app1... OK"})
	})
}

func TestRackResourcesUnlinkError(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("SystemGet").Return(fxSystem(), nil)
		i.On("SystemResourceUnlink", "resource1", "app1").Return(nil, fmt.Errorf("err1"))

		res, err := testExecute(e, "rack resources unlink resource1 -a app1", nil)
		require.NoError(t, err)
		require.Equal(t, 1, res.Code)
		res.RequireStderr(t, []string{"ERROR: err1"})
		res.RequireStdout(t, []string{"Unlinking from app1... "})
	})
}

func TestRackResourcesUpdate(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("SystemGet").Return(fxSystem(), nil)
		opts := structs.ResourceUpdateOptions{Parameters: map[string]string{"Foo": "bar", "Baz": "quux"}}
		i.On("SystemResourceUpdate", "resource1", opts).Return(fxResource(), nil)
		i.On("SystemResourceGet", "resource1").Return(fxResource(), nil).Once()
		i.On("SystemResourceGet", "resource1").Return(fxResourceUpdating(), nil).Twice()
		i.On("SystemResourceGet", "resource1").Return(fxResource(), nil)

		res, err := testExecute(e, "rack resources update resource1 Foo=bar Baz=quux", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		res.RequireStderr(t, []string{""})
		res.RequireStdout(t, []string{"Updating resource... OK"})
	})
}

func TestRackResourcesUpdateError(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("SystemGet").Return(fxSystem(), nil)
		opts := structs.ResourceUpdateOptions{Parameters: map[string]string{"Foo": "bar", "Baz": "quux"}}
		i.On("SystemResourceUpdate", "resource1", opts).Return(nil, fmt.Errorf("err1"))

		res, err := testExecute(e, "rack resources update resource1 Foo=bar Baz=quux", nil)
		require.NoError(t, err)
		require.Equal(t, 1, res.Code)
		res.RequireStderr(t, []string{"ERROR: err1"})
		res.RequireStdout(t, []string{"Updating resource... "})
	})
}

func TestRackResourcesUpdateClassic(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("SystemGet").Return(fxSystemClassic(), nil)
		opts := structs.ResourceUpdateOptions{Parameters: map[string]string{"Foo": "bar", "Baz": "quux"}}
		i.On("ResourceUpdateClassic", "resource1", opts).Return(fxResource(), nil)
		i.On("SystemResourceGetClassic", "resource1").Return(fxResource(), nil).Once()
		i.On("SystemResourceGetClassic", "resource1").Return(fxResourceUpdating(), nil).Twice()
		i.On("SystemResourceGetClassic", "resource1").Return(fxResource(), nil)

		res, err := testExecute(e, "rack resources update resource1 Foo=bar Baz=quux", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		res.RequireStderr(t, []string{""})
		res.RequireStdout(t, []string{"Updating resource... OK"})
	})
}

func TestRackResourcesUrl(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("SystemGet").Return(fxSystem(), nil)
		i.On("SystemResourceGet", "resource1").Return(fxResource(), nil)

		res, err := testExecute(e, "rack resources url resource1", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		res.RequireStderr(t, []string{""})
		res.RequireStdout(t, []string{"https://example.org/path"})
	})
}

func TestRackResourcesUrlError(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("SystemGet").Return(fxSystem(), nil)
		i.On("SystemResourceGet", "resource1").Return(nil, fmt.Errorf("err1"))

		res, err := testExecute(e, "rack resources url resource1", nil)
		require.NoError(t, err)
		require.Equal(t, 1, res.Code)
		res.RequireStderr(t, []string{"ERROR: err1"})
		res.RequireStdout(t, []string{""})
	})
}

func TestRackResourcesUrlClassic(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		i.On("SystemGet").Return(fxSystemClassic(), nil)
		i.On("SystemResourceGetClassic", "resource1").Return(fxResource(), nil)

		res, err := testExecute(e, "rack resources url resource1", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		res.RequireStderr(t, []string{""})
		res.RequireStdout(t, []string{"https://other.example.org/path"})
	})
}
