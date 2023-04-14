package cli_test

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/convox/convox/pkg/cli"
	mocksdk "github.com/convox/convox/pkg/mock/sdk"
	"github.com/convox/convox/pkg/structs"
	"github.com/stretchr/testify/require"
)

func TestSsoConfigure(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		res, err := testExecute(e,
			fmt.Sprintf("sso configure -p %s -c %s -s %s -i %s -u %s", "okta", "clientID", "clientSecret", "issuerURL", "callbackURL"),
			nil,
		)
		require.NoError(t, err)
		require.Equal(t, 0, res.Code)
		res.RequireStderr(t, []string{""})
		res.RequireStdout(t, []string{"OK"})

		data, err := ioutil.ReadFile(filepath.Join(e.Settings, "sso"))
		require.NoError(t, err)
		require.Equal(t,
			"{\n  \"callback_url\": \"callbackURL\",\n  \"client_id\": \"clientID\",\n  \"client_secret\": \"clientSecret\",\n  \"issuer\": \"issuerURL\",\n  \"provider\": \"okta\"\n}",
			string(data),
		)
	})
}

func TestAuthCodeCallbackHandler(t *testing.T) {
	testClient(t, func(e *cli.Engine, i *mocksdk.Interface) {
		w := httptest.NewRecorder()
		r, err := http.NewRequest("GET", "/callback?code=abc123&state=state123", nil)
		if err != nil {
			t.Fatalf("Error creating request: %v", err)
		}

		mockProvider := &MockSsoProvider{}

		// Call the authCodeCallbackHandler function with the done channel
		token := cli.AuthCodeCallbackHandler(w, r, mockProvider, openAuthHTMLFile)
		require.Equal(t,
			"token",
			token,
		)
	})
}

type MockSsoProvider struct {
}

func openAuthHTMLFile() ([]byte, error) {
	return []byte("<html><body>Mock authentication success</body></html>"), nil
}

func (m *MockSsoProvider) ExchangeCode(r *http.Request, code string) structs.SsoExchangeCode {
	return structs.SsoExchangeCode{
		Error:       "",
		AccessToken: "token",
	}
}

func (m *MockSsoProvider) Name() string {
	return "name"
}

func (m *MockSsoProvider) Opts() structs.SsoProviderOptions {
	return structs.SsoProviderOptions{
		State: "state123",
	}
}

func (m *MockSsoProvider) RedirectPath() string {
	return "provider_path"
}

func (m *MockSsoProvider) VerifyToken(t string) error {
	return nil
}
