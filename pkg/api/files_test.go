package api_test

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/convox/convox/pkg/api"
	"github.com/convox/convox/pkg/structs"
	"github.com/convox/stdapi"
	"github.com/convox/stdsdk"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestFilesDelete(t *testing.T) {
	testServer(t, func(c *stdsdk.Client, p *structs.MockProvider) {
		opts := stdsdk.RequestOptions{
			Query: stdsdk.Query{
				"files": "file1,file2",
			},
		}
		p.On("FilesDelete", "app1", "pid1", []string{"file1", "file2"}).Return(nil)
		err := c.Delete("/apps/app1/processes/pid1/files", opts, nil)
		require.NoError(t, err)
	})
}

func TestFilesDeleteError(t *testing.T) {
	testServer(t, func(c *stdsdk.Client, p *structs.MockProvider) {
		opts := stdsdk.RequestOptions{
			Query: stdsdk.Query{
				"files": "file1,file2",
			},
		}
		p.On("FilesDelete", "app1", "pid1", []string{"file1", "file2"}).Return(fmt.Errorf("err1"))
		err := c.Delete("/apps/app1/processes/pid1/files", opts, nil)
		require.EqualError(t, err, "err1")
	})
}

func TestFilesDownload(t *testing.T) {
	testServer(t, func(c *stdsdk.Client, p *structs.MockProvider) {
		r1 := strings.NewReader("data")
		opts := stdsdk.RequestOptions{
			Query: stdsdk.Query{
				"file": "file1",
			},
		}
		p.On("FilesDownload", "app1", "pid1", "file1").Return(r1, nil)
		res, err := c.GetStream("/apps/app1/processes/pid1/files", opts)
		require.NoError(t, err)
		defer res.Body.Close()
		data, err := io.ReadAll(res.Body)
		require.NoError(t, err)
		require.Equal(t, "data", string(data))
	})
}

func TestFilesDownloadError(t *testing.T) {
	testServer(t, func(c *stdsdk.Client, p *structs.MockProvider) {
		opts := stdsdk.RequestOptions{
			Query: stdsdk.Query{
				"file": "file1",
			},
		}
		p.On("FilesDownload", "app1", "pid1", "file1").Return(nil, fmt.Errorf("err1"))
		res, err := c.GetStream("/apps/app1/processes/pid1/files", opts)
		require.EqualError(t, err, "err1")
		require.Nil(t, res)
	})
}

func TestFilesDownloadRequiresWrite(t *testing.T) {
	s := &api.Server{}

	c := stdapi.NewContext(nil, httptest.NewRequest(http.MethodGet, "http://example.org", nil))
	api.SetReadRole(c)

	err := s.FilesDownload(c)
	require.EqualError(t, err, "file download requires write access")

	aerr, ok := err.(stdapi.Error)
	require.True(t, ok)
	require.Equal(t, http.StatusForbidden, aerr.Code())
}

func TestFilesDownloadAllowsWriteAndAdmin(t *testing.T) {
	for _, setRole := range []func(*stdapi.Context){api.SetReadWriteRole, api.SetAdminRole} {
		p := &structs.MockProvider{}
		p.On("WithContext", mock.Anything).Return(p)
		p.On("FilesDownload", "app1", "pid1", "file1").Return(strings.NewReader("data"), nil)

		s := &api.Server{Provider: p}

		w := httptest.NewRecorder()
		c := stdapi.NewContext(w, httptest.NewRequest(http.MethodGet, "http://example.org/?file=file1", nil))
		c.SetVar("app", "app1")
		c.SetVar("pid", "pid1")
		setRole(c)

		require.NoError(t, s.FilesDownload(c))
		require.Equal(t, "data", w.Body.String())

		p.AssertExpectations(t)
	}
}

func TestFilesUpload(t *testing.T) {
	testServer(t, func(c *stdsdk.Client, p *structs.MockProvider) {
		opts := stdsdk.RequestOptions{
			Body: strings.NewReader("data"),
		}
		p.On("FilesUpload", "app1", "pid1", mock.Anything, mock.Anything).Return(nil).Run(func(args mock.Arguments) {
			data, err := io.ReadAll(args.Get(2).(io.Reader)) //nolint:errcheck // mock type assertion
			require.NoError(t, err)
			require.Equal(t, "data", string(data))
		})
		err := c.Post("/apps/app1/processes/pid1/files", opts, nil)
		require.NoError(t, err)
	})
}

func TestFilesUploadError(t *testing.T) {
	testServer(t, func(c *stdsdk.Client, p *structs.MockProvider) {
		p.On("FilesUpload", "app1", "pid1", mock.Anything, mock.Anything).Return(fmt.Errorf("err1"))
		err := c.Post("/apps/app1/processes/pid1/files", stdsdk.RequestOptions{}, nil)
		require.EqualError(t, err, "err1")
	})
}
