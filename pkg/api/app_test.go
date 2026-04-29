package api_test

import (
	"bytes"
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/convox/convox/pkg/options"
	"github.com/convox/convox/pkg/structs"
	"github.com/convox/stdsdk"
	"github.com/stretchr/testify/require"
)

var fxApp = structs.App{
	Generation: "generation",
	Name:       "name",
	Release:    "release1",
	Status:     "created",
	Parameters: map[string]string{
		"p1": "v1",
		"p2": "v2",
	},
}

func TestAppCancel(t *testing.T) {
	testServer(t, func(c *stdsdk.Client, p *structs.MockProvider) {
		p.On("AppGet", "app1").Return(&structs.App{Status: "updating"}, nil)
		p.On("AppCancel", "app1").Return(nil)
		err := c.Post("/apps/app1/cancel", stdsdk.RequestOptions{}, nil)
		require.NoError(t, err)
	})
}

func TestAppCancelError(t *testing.T) {
	testServer(t, func(c *stdsdk.Client, p *structs.MockProvider) {
		p.On("AppGet", "app1").Return(&structs.App{Status: "updating"}, nil)
		p.On("AppCancel", "app1").Return(fmt.Errorf("err1"))
		err := c.Post("/apps/app1/cancel", stdsdk.RequestOptions{}, nil)
		require.EqualError(t, err, "err1")
	})
}

func TestAppCancelValidateNotUpdating(t *testing.T) {
	testServer(t, func(c *stdsdk.Client, p *structs.MockProvider) {
		p.On("AppGet", "app1").Return(&structs.App{Status: "running"}, nil)
		err := c.Post("/apps/app1/cancel", stdsdk.RequestOptions{}, nil)
		require.EqualError(t, err, "app is not updating")
	})
}

func TestAppCancelValidateError(t *testing.T) {
	testServer(t, func(c *stdsdk.Client, p *structs.MockProvider) {
		p.On("AppGet", "app1").Return(nil, fmt.Errorf("err1"))
		err := c.Post("/apps/app1/cancel", stdsdk.RequestOptions{}, nil)
		require.EqualError(t, err, "err1")
	})
}

func TestAppCreate(t *testing.T) {
	testServer(t, func(c *stdsdk.Client, p *structs.MockProvider) {
		a1 := fxApp
		a2 := structs.App{}
		opts := structs.AppCreateOptions{
			Generation: options.String("2"),
		}
		ro := stdsdk.RequestOptions{
			Params: stdsdk.Params{
				"name": "app1",
			},
		}
		p.On("AppCreate", "app1", opts).Return(&a1, nil)
		err := c.Post("/apps", ro, &a2)
		require.NoError(t, err)
		require.Equal(t, a1, a2)
	})
}

func TestAppCreateError(t *testing.T) {
	testServer(t, func(c *stdsdk.Client, p *structs.MockProvider) {
		var a1 *structs.App
		opts := structs.AppCreateOptions{
			Generation: options.String("2"),
		}
		ro := stdsdk.RequestOptions{
			Params: stdsdk.Params{
				"name": "app1",
			},
		}
		p.On("AppCreate", "app1", opts).Return(nil, fmt.Errorf("err1"))
		err := c.Post("/apps", ro, a1)
		require.EqualError(t, err, "err1")
		require.Nil(t, a1)
	})
}

func TestAppCreateGeneration1(t *testing.T) {
	testServer(t, func(c *stdsdk.Client, p *structs.MockProvider) {
		a1 := fxApp
		a2 := structs.App{}
		opts := structs.AppCreateOptions{
			Generation: options.String("1"),
		}
		ro := stdsdk.RequestOptions{
			Params: stdsdk.Params{
				"generation": "1",
				"name":       "app1",
			},
		}
		p.On("AppCreate", "app1", opts).Return(&a1, nil)
		err := c.Post("/apps", ro, &a2)
		require.NoError(t, err)
		require.Equal(t, a1, a2)
	})
}

func TestAppDelete(t *testing.T) {
	testServer(t, func(c *stdsdk.Client, p *structs.MockProvider) {
		p.On("AppDelete", "app1").Return(nil)
		err := c.Delete("/apps/app1", stdsdk.RequestOptions{}, nil)
		require.NoError(t, err)
	})
}

func TestAppDeleteError(t *testing.T) {
	testServer(t, func(c *stdsdk.Client, p *structs.MockProvider) {
		p.On("AppDelete", "app1").Return(fmt.Errorf("err1"))
		err := c.Delete("/apps/app1", stdsdk.RequestOptions{}, nil)
		require.EqualError(t, err, "err1")
	})
}

func TestAppGet(t *testing.T) {
	testServer(t, func(c *stdsdk.Client, p *structs.MockProvider) {
		a1 := fxApp
		a2 := structs.App{}
		p.On("AppGet", "app1").Return(&a1, nil)
		err := c.Get("/apps/app1", stdsdk.RequestOptions{}, &a2)
		require.NoError(t, err)
		require.Equal(t, a1, a2)
	})
}

func TestAppGetError(t *testing.T) {
	testServer(t, func(c *stdsdk.Client, p *structs.MockProvider) {
		var a1 *structs.App
		p.On("AppGet", "app1").Return(nil, fmt.Errorf("err1"))
		err := c.Get("/apps/app1", stdsdk.RequestOptions{}, a1)
		require.EqualError(t, err, "err1")
		require.Nil(t, a1)
	})
}

func TestAppList(t *testing.T) {
	testServer(t, func(c *stdsdk.Client, p *structs.MockProvider) {
		a1 := structs.Apps{fxApp, fxApp}
		a2 := structs.Apps{}
		p.On("AppList").Return(a1, nil)
		err := c.Get("/apps", stdsdk.RequestOptions{}, &a2)
		require.NoError(t, err)
		require.Equal(t, a1, a2)
	})
}

func TestAppListError(t *testing.T) {
	testServer(t, func(c *stdsdk.Client, p *structs.MockProvider) {
		var a1 structs.Apps
		p.On("AppList").Return(nil, fmt.Errorf("err1"))
		err := c.Get("/apps", stdsdk.RequestOptions{}, &a1)
		require.EqualError(t, err, "err1")
		require.Nil(t, a1)
	})
}

func TestAppLogs(t *testing.T) {
	testServer(t, func(c *stdsdk.Client, p *structs.MockProvider) {
		d1 := []byte("test")
		r1 := io.NopCloser(bytes.NewReader(d1))
		opts := structs.LogsOptions{Since: options.Duration(2 * time.Minute)}
		p.On("AppLogs", "app1", opts).Return(r1, nil)
		r2, err := c.Websocket("/apps/app1/logs", stdsdk.RequestOptions{})
		require.NoError(t, err)
		d2, err := io.ReadAll(r2)
		require.NoError(t, err)
		require.Equal(t, d1, d2)
	})
}

func TestAppLogsMaxLogRequests(t *testing.T) {
	testServer(t, func(c *stdsdk.Client, p *structs.MockProvider) {
		d1 := []byte("test")
		r1 := io.NopCloser(bytes.NewReader(d1))
		opts := structs.LogsOptions{Since: options.Duration(2 * time.Minute), MaxLogRequests: options.Int(50)}
		p.On("AppLogs", "app1", opts).Return(r1, nil)
		r2, err := c.Websocket("/apps/app1/logs", stdsdk.RequestOptions{
			Headers: stdsdk.Headers{"Maxlogrequests": "50"},
		})
		require.NoError(t, err)
		d2, err := io.ReadAll(r2)
		require.NoError(t, err)
		require.Equal(t, d1, d2)
	})
}

func TestAppLogsError(t *testing.T) {
	testServer(t, func(c *stdsdk.Client, p *structs.MockProvider) {
		opts := structs.LogsOptions{Since: options.Duration(2 * time.Minute)}
		p.On("AppLogs", "app1", opts).Return(nil, fmt.Errorf("err1"))
		r1, err := c.Websocket("/apps/app1/logs", stdsdk.RequestOptions{})
		require.NoError(t, err)
		require.NotNil(t, r1)
		d1, err := io.ReadAll(r1)
		require.NoError(t, err)
		require.Equal(t, []byte("ERROR: err1\n"), d1)
	})
}

// TestAppManifestService — item 24 happy path. The new
// /apps/{app}/manifest/services/{service} route delegates to the provider
// and renders the ManifestService transport struct as JSON.
func TestAppManifestService(t *testing.T) {
	testServer(t, func(c *stdsdk.Client, p *structs.MockProvider) {
		min, max := 1, 5
		ms := &structs.ManifestService{
			Name:        "api",
			Environment: []string{"FOO=bar", "BAZ=qux"},
			Scale:       &structs.ManifestServiceScale{Min: &min, Max: &max},
		}
		p.On("AppManifestService", "app1", "api").Return(ms, nil)

		var got structs.ManifestService
		err := c.Get("/apps/app1/manifest/services/api", stdsdk.RequestOptions{}, &got)
		require.NoError(t, err)
		require.Equal(t, "api", got.Name)
		require.Equal(t, []string{"FOO=bar", "BAZ=qux"}, got.Environment)
		require.NotNil(t, got.Scale)
		require.NotNil(t, got.Scale.Min)
		require.Equal(t, 1, *got.Scale.Min)
		require.NotNil(t, got.Scale.Max)
		require.Equal(t, 5, *got.Scale.Max)
	})
}

// TestAppManifestServiceNoScale — service with no scale block returns
// Scale=nil; omitempty drops the field from JSON.
func TestAppManifestServiceNoScale(t *testing.T) {
	testServer(t, func(c *stdsdk.Client, p *structs.MockProvider) {
		ms := &structs.ManifestService{
			Name:        "worker",
			Environment: []string{"WORKER_QUEUE=default"},
		}
		p.On("AppManifestService", "app1", "worker").Return(ms, nil)

		var got structs.ManifestService
		err := c.Get("/apps/app1/manifest/services/worker", stdsdk.RequestOptions{}, &got)
		require.NoError(t, err)
		require.Equal(t, "worker", got.Name)
		require.Equal(t, []string{"WORKER_QUEUE=default"}, got.Environment)
		require.Nil(t, got.Scale)
	})
}

// TestAppManifestServiceError — provider error propagates as HTTP error to
// caller. Covers both the service-not-found case (the only synthetic path
// in the K8s provider implementation) and any underlying common.AppManifest
// failure (no release, AppGet error).
func TestAppManifestServiceError(t *testing.T) {
	testServer(t, func(c *stdsdk.Client, p *structs.MockProvider) {
		p.On("AppManifestService", "app1", "missing").Return(nil, fmt.Errorf("service missing not found in manifest for app app1"))
		err := c.Get("/apps/app1/manifest/services/missing", stdsdk.RequestOptions{}, nil)
		require.EqualError(t, err, "service missing not found in manifest for app app1")
	})
}

func TestAppUpdate(t *testing.T) {
	testServer(t, func(c *stdsdk.Client, p *structs.MockProvider) {
		opts := structs.AppUpdateOptions{
			Parameters: map[string]string{
				"foo": "bar",
				"baz": "qux",
			},
		}
		ro := stdsdk.RequestOptions{
			Params: stdsdk.Params{
				"parameters": "foo=bar&baz=qux",
				"sleep":      "true",
			},
		}
		p.On("AppUpdate", "app1", opts).Return(nil)
		err := c.Put("/apps/app1", ro, nil)
		require.NoError(t, err)
	})
}

func TestAppUpdateError(t *testing.T) {
	testServer(t, func(c *stdsdk.Client, p *structs.MockProvider) {
		p.On("AppUpdate", "app1", structs.AppUpdateOptions{}).Return(fmt.Errorf("err1"))
		err := c.Put("/apps/app1", stdsdk.RequestOptions{}, nil)
		require.EqualError(t, err, "err1")
	})
}
