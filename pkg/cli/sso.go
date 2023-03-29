package cli

import (
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/convox/convox/pkg/structs"
	"github.com/convox/convox/sdk"
	"github.com/convox/convox/sso"
	"github.com/convox/stdcli"
)

func init() {
	registerWithoutProvider("sso configure", "configure sso config file", SsoConfigure, stdcli.CommandOptions{
		Flags: []stdcli.Flag{
			stdcli.StringFlag("provider", "p", "sso provider"),
			stdcli.StringFlag("client_id", "c", "client id"),
			stdcli.StringFlag("client_secret", "s", "client secret"),
			stdcli.StringFlag("issuer", "i", "issuer"),
		},
		Validate: stdcli.ArgsMax(0),
	})

	registerWithoutProvider("sso login", "authenticate with a console using sso", SsoLogin, stdcli.CommandOptions{
		Validate: stdcli.ArgsMax(0),
	})
}

func SsoConfigure(rack sdk.Interface, c *stdcli.Context) error {
	var err error
	provider := coalesce(c.String("provider"), os.Getenv("SSO_PROVIDER"))
	if provider == "" {
		c.Writef("SSO Provider: ")

		provider, err = c.ReadSecret()
		if err != nil {
			return err
		}

		c.Writef("\n")
	}

	clientID := coalesce(c.String("client_id"), os.Getenv("SSO_CLIENT_ID"))
	if clientID == "" {
		c.Writef("SSO Client ID: ")

		clientID, err = c.ReadSecret()
		if err != nil {
			return err
		}

		c.Writef("\n")
	}

	clientSecret := coalesce(c.String("client_secret"), os.Getenv("SSO_CLIENT_SECRET"))
	if clientSecret == "" {
		c.Writef("SSO Client Secret: ")

		clientSecret, err = c.ReadSecret()
		if err != nil {
			return err
		}

		c.Writef("\n")
	}

	issuer := coalesce(c.String("issuer"), os.Getenv("SSO_ISSUER"))
	if issuer == "" {
		c.Writef("SSO ISSUER: ")

		issuer, err = c.ReadSecret()
		if err != nil {
			return err
		}

		c.Writef("\n")
	}

	if err := c.SettingWriteKey("sso", "provider", provider); err != nil {
		return err
	}

	if err := c.SettingWriteKey("sso", "client_id", clientID); err != nil {
		return err
	}

	if err := c.SettingWriteKey("sso", "client_secret", clientSecret); err != nil {
		return err
	}

	if err := c.SettingWriteKey("sso", "issuer", issuer); err != nil {
		return err
	}

	return c.OK()
}

func SsoLogin(rack sdk.Interface, c *stdcli.Context) error {
	provider, err := c.SettingReadKey("sso", "provider")
	if err != nil {
		return err
	}

	clientID, err := c.SettingReadKey("sso", "client_id")
	if err != nil {
		return err
	}

	clientSecret, err := c.SettingReadKey("sso", "client_secret")
	if err != nil {
		return err
	}

	issuer, err := c.SettingReadKey("sso", "issuer")
	if err != nil {
		return err
	}

	nonce, _ := sso.GenerateNonce()

	p, err := sso.Initialize(provider, structs.SsoProviderOptions{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Issuer:       issuer,
		Scope:        "openid profile email",
		State:        sso.GenerateState(),
		Nonce:        nonce,
	})

	if err != nil {
		return err
	}

	server := &http.Server{Addr: "/authorization-code/callback"}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		authCodeCallbackHandler(w, r, server, c, p)
	})

	port := fmt.Sprintf(":%s", "8090")
	l, err := net.Listen("tcp", port)
	if err != nil {
		fmt.Printf("snap: can't listen to port %s: %s\n", port, err)
		os.Exit(1)
	}

	log.Println("Waiting for login... ")

	time.Sleep(1 * time.Second)
	sso.Openbrowser(p.RedirectPath())
	time.Sleep(1 * time.Second)

	server.Serve(l)

	return c.OK()
}

func authCodeCallbackHandler(w http.ResponseWriter, r *http.Request, server *http.Server, c *stdcli.Context, p structs.SsoProvider) {
	if r.URL.Query().Get("state") != p.Opts().State {
		fmt.Fprintln(w, "The state was not as expected")
		return
	}

	if r.URL.Query().Get("code") == "" {
		fmt.Fprintln(w, "The code was not returned or is not accessible")
		return
	}

	exchange := p.ExchangeCode(r, r.URL.Query().Get("code"))
	if exchange.Error != "" {
		fmt.Fprintf(w, exchange.Error)
		fmt.Fprintf(w, exchange.ErrorDescription)
		return
	}

	verificationError := p.VerifyToken(exchange.IdToken)

	if verificationError != nil {
		fmt.Println(verificationError)
	}

	if err := c.SettingWriteKey("sso", "bearer_token", exchange.AccessToken); err != nil {
		fmt.Fprintf(w, "Could not set Bearer Token on the CLI settings")
		return
	}

	w.Header().Set("Content-Type", "text/html")
	w.Write(openSuccessHTMLFile())

	cleanup(server)
}

func openSuccessHTMLFile() []byte {
	file, err := os.Open("../../sso/templates/success.html")
	if err != nil {
		return nil
	}

	defer file.Close()

	fileContents, err := io.ReadAll(file)
	if err != nil {
		return nil
	}

	return fileContents
}

func cleanup(server *http.Server) {
	go server.Close()
}
