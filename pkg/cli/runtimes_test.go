package cli_test

import (
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"testing"

	"github.com/convox/convox/pkg/cli"
	mocksdk "github.com/convox/convox/pkg/mock/sdk"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/require"
)

func TestRuntimesSuccess(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		r := mux.NewRouter()

		r.HandleFunc("/organizations/test-org-stg/runtimes", func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(`[
				{"Id":"b29266a2-0d25-4194-b375-a7ac722f82a5","Title":"533267189958"},
			]`))
		}).Methods("GET")

		ts := httptest.NewTLSServer(r)

		tsu, err := url.Parse(ts.URL)
		require.NoError(t, err)

		err = ioutil.WriteFile(filepath.Join(e.Settings, "console"), []byte(tsu.Host), 0644)
		require.NoError(t, err)

		res, err := testExecute(e, "runtimes test-org-stg", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		res.RequireStderr(t, []string{""})
		res.RequireStdout(t, []string{
			"ID                                    TITLE",
			"b29266a2-0d25-4194-b375-a7ac722f82a5  533267189958",
		})
	})
}

func TestRuntimesNone(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		res, err := testExecute(e, "runtimes test-org-stg", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		res.RequireStderr(t, []string{""})
		res.RequireStdout(t, []string{
			"ID                                    TITLE",
		})
	})
}
