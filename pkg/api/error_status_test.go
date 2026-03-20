package api_test

// HTTP contract tests for error status codes and response formats.
//
// These tests verify the API's error contract using raw HTTP requests (bypassing
// the SDK client) to ensure SDK consumers (e.g., convox-python) receive correct
// HTTP status codes and JSON error bodies. The SDK client strips status codes and
// only returns error message strings, so these tests hit the server directly.

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/convox/convox/pkg/api"
	"github.com/convox/convox/pkg/structs"
	"github.com/convox/logger"
	"github.com/convox/stdapi"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// testHTTPServer creates a test API server and calls fn with the raw server URL
// and mock provider. Unlike testServer, this exposes the URL for making direct
// HTTP requests to verify the HTTP contract independently of the SDK client.
func testHTTPServer(t *testing.T, fn func(serverURL string, p *structs.MockProvider)) {
	t.Helper()

	p := &structs.MockProvider{}
	p.On("Initialize", mock.Anything).Return(nil)
	p.On("Start").Return(nil)
	p.On("WithContext", mock.Anything).Return(p).Maybe()
	p.On("SystemJwtSignKey").Return("test", nil)

	s := api.NewWithProvider(p)
	s.Logger = logger.Discard
	s.Server.Recover = func(err error, c *stdapi.Context) {
		require.NoError(t, err, "httptest server panic")
	}

	ht := httptest.NewServer(s)
	defer ht.Close()

	fn(ht.URL, p)

	p.AssertExpectations(t)
}

func TestErrorStatusCodes(t *testing.T) {
	tests := []struct {
		name        string
		providerErr error
		wantStatus  int
		wantMsg     string
	}{
		{
			name:        "not found returns 404",
			providerErr: structs.ErrNotFound("app not found: nonexistent"),
			wantStatus:  http.StatusNotFound,
			wantMsg:     "app not found: nonexistent",
		},
		{
			name:        "bad request returns 400",
			providerErr: structs.ErrBadRequest("app name is reserved"),
			wantStatus:  http.StatusBadRequest,
			wantMsg:     "app name is reserved",
		},
		{
			name:        "conflict returns 409",
			providerErr: structs.ErrConflict("app already exists: myapp"),
			wantStatus:  http.StatusConflict,
			wantMsg:     "app already exists: myapp",
		},
		{
			name:        "not implemented returns 501",
			providerErr: structs.ErrNotImplemented("unimplemented"),
			wantStatus:  http.StatusNotImplemented,
			wantMsg:     "unimplemented",
		},
		{
			name:        "untyped error returns 500",
			providerErr: fmt.Errorf("something broke"),
			wantStatus:  http.StatusInternalServerError,
			wantMsg:     "something broke",
		},
		{
			name:        "wrapped typed error preserves status code",
			providerErr: errors.WithStack(structs.ErrNotFound("app not found: wrapped")),
			wantStatus:  http.StatusNotFound,
			wantMsg:     "app not found: wrapped",
		},
		{
			name:        "wrapped untyped error returns 500",
			providerErr: errors.WithStack(fmt.Errorf("wrapped internal")),
			wantStatus:  http.StatusInternalServerError,
			wantMsg:     "wrapped internal",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testHTTPServer(t, func(serverURL string, p *structs.MockProvider) {
				p.On("AppGet", "testapp").Return(nil, tt.providerErr)

				resp, err := http.Get(serverURL + "/apps/testapp")
				require.NoError(t, err)
				defer resp.Body.Close()

				require.Equal(t, tt.wantStatus, resp.StatusCode,
					"expected status %d for %s, got %d", tt.wantStatus, tt.name, resp.StatusCode)

				body, err := io.ReadAll(resp.Body)
				require.NoError(t, err)
				require.Contains(t, string(body), tt.wantMsg)
			})
		})
	}
}

func TestErrorResponseJSON(t *testing.T) {
	testHTTPServer(t, func(serverURL string, p *structs.MockProvider) {
		p.On("AppGet", "testapp").Return(nil, structs.ErrNotFound("app not found: testapp"))

		req, err := http.NewRequest("GET", serverURL+"/apps/testapp", nil)
		require.NoError(t, err)
		req.Header.Set("Accept", "application/json")

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		require.Equal(t, http.StatusNotFound, resp.StatusCode)
		require.Equal(t, "application/json", resp.Header.Get("Content-Type"))

		var errResp map[string]string
		err = json.NewDecoder(resp.Body).Decode(&errResp)
		require.NoError(t, err)
		require.Equal(t, "app not found: testapp", errResp["error"])
	})
}

func TestErrorResponseJSONWithQuality(t *testing.T) {
	testHTTPServer(t, func(serverURL string, p *structs.MockProvider) {
		p.On("AppGet", "testapp").Return(nil, structs.ErrNotFound("app not found: testapp"))

		req, err := http.NewRequest("GET", serverURL+"/apps/testapp", nil)
		require.NoError(t, err)
		req.Header.Set("Accept", "text/html, application/json;q=0.9")

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		require.Equal(t, http.StatusNotFound, resp.StatusCode)
		require.Equal(t, "application/json", resp.Header.Get("Content-Type"))

		var errResp map[string]string
		err = json.NewDecoder(resp.Body).Decode(&errResp)
		require.NoError(t, err)
		require.Equal(t, "app not found: testapp", errResp["error"])
	})
}

func TestErrorResponsePlainText(t *testing.T) {
	testHTTPServer(t, func(serverURL string, p *structs.MockProvider) {
		p.On("AppGet", "testapp").Return(nil, structs.ErrNotFound("app not found: testapp"))

		resp, err := http.Get(serverURL + "/apps/testapp")
		require.NoError(t, err)
		defer resp.Body.Close()

		require.Equal(t, http.StatusNotFound, resp.StatusCode)
		require.Contains(t, resp.Header.Get("Content-Type"), "text/plain")

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		require.Contains(t, string(body), "app not found: testapp")
	})
}

func TestErrorResponseNoAcceptHeader(t *testing.T) {
	testHTTPServer(t, func(serverURL string, p *structs.MockProvider) {
		p.On("AppGet", "testapp").Return(nil, structs.ErrBadRequest("invalid input"))

		resp, err := http.Get(serverURL + "/apps/testapp")
		require.NoError(t, err)
		defer resp.Body.Close()

		require.Equal(t, http.StatusBadRequest, resp.StatusCode)
		require.Contains(t, resp.Header.Get("Content-Type"), "text/plain")
	})
}
