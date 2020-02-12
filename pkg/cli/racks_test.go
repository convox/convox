package cli_test

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"testing"

	"github.com/convox/convox/pkg/cli"
	mocksdk "github.com/convox/convox/pkg/mock/sdk"
	mockstdcli "github.com/convox/convox/pkg/mock/stdcli"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/require"
)

func TestRacksNone(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		res, err := testExecute(e, "racks", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		res.RequireStderr(t, []string{""})
		res.RequireStdout(t, []string{
			"NAME  PROVIDER  STATUS",
		})
	})
}

func TestRacksLocal(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		require.NoError(t, testLocalRack(e, "dev1", "local", "https://host1"))

		res, err := testExecute(e, "racks", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		res.RequireStderr(t, []string{""})
		res.RequireStdout(t, []string{
			"NAME  PROVIDER  STATUS ",
			"dev1  local     running",
		})
	})
}

func TestRacksRemote(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		r := mux.NewRouter()

		r.HandleFunc("/racks", func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(`[
				{"name":"foo","organization":{"name":"test"},"provider":"prov1","status":"running"},
				{"name":"other","organization":{"name":"test"},"provider":"prov2","status":"updating"}
			]`))
		}).Methods("GET")

		ts := httptest.NewTLSServer(r)

		tsu, err := url.Parse(ts.URL)
		require.NoError(t, err)

		err = ioutil.WriteFile(filepath.Join(e.Settings, "console"), []byte(tsu.Host), 0644)
		require.NoError(t, err)

		res, err := testExecute(e, "racks", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		res.RequireStderr(t, []string{""})
		res.RequireStdout(t, []string{
			"NAME        PROVIDER  STATUS  ",
			"test/foo    prov1     running ",
			"test/other  prov2     updating",
		})
	})
}

func TestRacksLocalAndRemote(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		require.NoError(t, testLocalRack(e, "dev1", "local", "https://host1"))

		r := mux.NewRouter()

		r.HandleFunc("/racks", func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(`[
				{"name":"foo","organization":{"name":"test"},"provider":"prov1","status":"running"},
				{"name":"other","organization":{"name":"test"},"status":"updating"}
			]`))
		}).Methods("GET")

		ts := httptest.NewTLSServer(r)

		tsu, err := url.Parse(ts.URL)
		require.NoError(t, err)

		err = ioutil.WriteFile(filepath.Join(e.Settings, "console"), []byte(tsu.Host), 0644)
		require.NoError(t, err)

		res, err := testExecute(e, "racks", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		res.RequireStderr(t, []string{""})
		res.RequireStdout(t, []string{
			"NAME        PROVIDER  STATUS  ",
			"dev1        local     running ",
			"test/foo    prov1     running ",
			"test/other  unknown   updating",
		})
	})
}

func TestRacksError(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		r := mux.NewRouter()

		r.HandleFunc("/racks", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(500)
			w.Write([]byte("test"))
		}).Methods("GET")

		ts := httptest.NewTLSServer(r)

		tsu, err := url.Parse(ts.URL)
		require.NoError(t, err)

		err = ioutil.WriteFile(filepath.Join(e.Settings, "host"), []byte(tsu.Host), 0644)
		require.NoError(t, err)

		me := &mockstdcli.Executor{}
		me.On("Execute", "kubectl", "get", "ns", "--selector=system=convox,type=rack", "--output=name").Return(nil, fmt.Errorf("err1"))
		e.Executor = me

		res, err := testExecute(e, "racks", nil)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		res.RequireStderr(t, []string{""})
		res.RequireStdout(t, []string{
			"NAME  PROVIDER  STATUS",
		})
	})
}
