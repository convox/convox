package api_test

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/convox/convox/pkg/api"
	"github.com/convox/convox/pkg/structs"
	"github.com/convox/stdapi"
	"github.com/convox/stdsdk"
	"github.com/stretchr/testify/require"
)

func TestRejectReservedApp(t *testing.T) {
	s := &api.Server{}

	call := func(app string) (error, bool) {
		c := stdapi.NewContext(nil, httptest.NewRequest(http.MethodGet, "http://example.org", nil))
		c.SetVar("app", app)

		called := false
		err := s.RejectReservedApp(func(c *stdapi.Context) error {
			called = true
			return nil
		})(c)

		return err, called
	}

	for _, app := range []string{"system", "rack"} {
		err, called := call(app)
		require.EqualError(t, err, "app name is reserved")
		aerr, ok := err.(stdapi.Error)
		require.True(t, ok)
		require.Equal(t, http.StatusBadRequest, aerr.Code())
		require.False(t, called)
	}

	for _, app := range []string{"app1", ""} {
		err, called := call(app)
		require.NoError(t, err)
		require.True(t, called)
	}
}

func TestReservedAppRejectedOnHTTPRoutes(t *testing.T) {
	routes := []struct {
		method string
		path   string
	}{
		{"GET", "/apps/%s/processes/pid1/files"},
		{"POST", "/apps/%s/cancel"},
		{"DELETE", "/apps/%s"},
		{"GET", "/apps/%s"},
		{"GET", "/apps/%s/metrics"},
		{"PUT", "/apps/%s"},
	}

	for _, rt := range routes {
		for _, app := range []string{"system", "rack"} {
			t.Run(fmt.Sprintf("%s %s", rt.method, fmt.Sprintf(rt.path, app)), func(t *testing.T) {
				testServer(t, func(c *stdsdk.Client, p *structs.MockProvider) {
					path := fmt.Sprintf(rt.path, app)

					var err error

					switch rt.method {
					case "GET":
						var res *http.Response
						res, err = c.GetStream(path, stdsdk.RequestOptions{})
						if res != nil {
							defer res.Body.Close()
						}
					case "POST":
						err = c.Post(path, stdsdk.RequestOptions{}, nil)
					case "PUT":
						err = c.Put(path, stdsdk.RequestOptions{}, nil)
					case "DELETE":
						err = c.Delete(path, stdsdk.RequestOptions{}, nil)
					}

					require.EqualError(t, err, "app name is reserved")
				})
			})
		}
	}
}

func TestReservedAppRejectedOnSocketRoute(t *testing.T) {
	for _, app := range []string{"system", "rack"} {
		testServer(t, func(c *stdsdk.Client, p *structs.MockProvider) {
			r, err := c.Websocket(fmt.Sprintf("/apps/%s/logs", app), stdsdk.RequestOptions{})
			require.NoError(t, err)

			data, err := io.ReadAll(r)
			require.NoError(t, err)
			require.Equal(t, "ERROR: app name is reserved\n", string(data))
		})
	}
}

func TestReservedAppAllowsNormalNames(t *testing.T) {
	testServer(t, func(c *stdsdk.Client, p *structs.MockProvider) {
		p.On("AppGet", "app1").Return(&structs.App{Name: "app1", Status: "updating"}, nil)
		p.On("AppCancel", "app1").Return(nil)
		require.NoError(t, c.Post("/apps/app1/cancel", stdsdk.RequestOptions{}, nil))
	})

	testServer(t, func(c *stdsdk.Client, p *structs.MockProvider) {
		p.On("AppMetrics", "app1", structs.MetricsOptions{}).Return(structs.Metrics{}, nil)

		var m structs.Metrics
		require.NoError(t, c.Get("/apps/app1/metrics", stdsdk.RequestOptions{}, &m))
	})
}
