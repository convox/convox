package cli_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/convox/convox/pkg/cli"
	mocksdk "github.com/convox/convox/pkg/mock/sdk"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/require"
)

func TestRackParamsSetRotateAPIPasswordConsoleUnreachable(t *testing.T) {
	for _, tc := range []struct {
		name    string
		args    string
		release string
		err     string
	}{
		{name: "rotate", args: "rotate_api_password=true", release: "3.26.0"},
		{name: "other parameter", args: "telemetry=false", release: "3.26.0", err: "current version invalid"},
		{name: "no stored release", args: "rotate_api_password=true", err: "current version invalid"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
				var puts []url.Values

				r := mux.NewRouter()
				r.HandleFunc("/racks", func(w http.ResponseWriter, r *http.Request) {
					fmt.Fprint(w, `[{"name":"foo","organization":{"name":"test"},"provider":"aws","status":"unknown"}]`)
				}).Methods("GET")
				r.HandleFunc("/racks/test/foo", func(w http.ResponseWriter, r *http.Request) {
					fmt.Fprintf(w, `{"name":"foo","provider":"aws","status":"unknown","version":"","state":"e30=","parameters":{"release":%q}}`, tc.release)
				}).Methods("GET")
				r.HandleFunc("/racks/test/foo", func(w http.ResponseWriter, r *http.Request) {
					require.NoError(t, r.ParseForm())
					puts = append(puts, r.PostForm)
				}).Methods("PUT")

				ts := httptest.NewTLSServer(r)
				defer ts.Close()

				tsu, err := url.Parse(ts.URL)
				require.NoError(t, err)
				require.NoError(t, os.WriteFile(filepath.Join(e.Settings, "console"), []byte(tsu.Host), 0600))

				res, err := testExecute(e, "rack params set "+tc.args+" --rack test/foo", nil)
				require.NoError(t, err)

				if tc.err != "" {
					require.Equal(t, 1, res.Code)
					res.RequireStderr(t, []string{"ERROR: " + tc.err})
					require.Empty(t, puts)
					return
				}

				require.Equal(t, 0, res.Code)
				require.Len(t, puts, 1)
				require.Equal(t, tc.release, puts[0].Get("version"))
				require.JSONEq(t, `{"rotate_api_password":"true"}`, puts[0].Get("params"))
			})
		})
	}
}
