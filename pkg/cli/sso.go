package cli

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/convox/convox/pkg/structs"
	"github.com/convox/convox/sdk"
	"github.com/convox/convox/sso"
	"github.com/convox/stdcli"
	"github.com/gobuffalo/packr"
)

func init() {
	registerWithoutProvider("sso configure", "configure sso config file", SsoConfigure, stdcli.CommandOptions{
		Flags: []stdcli.Flag{
			stdcli.StringFlag("provider", "p", "sso provider"),
			stdcli.StringFlag("client_id", "c", "client id"),
			stdcli.StringFlag("client_secret", "s", "client secret"),
			stdcli.StringFlag("issuer", "i", "issuer"),
			stdcli.StringFlag("callback_url", "u", "callback url"),
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
		c.Writef("SSO Issuer: ")

		issuer, err = c.ReadSecret()
		if err != nil {
			return err
		}

		c.Writef("\n")
	}

	callbackURL := coalesce(c.String("callback_url"), os.Getenv("SSO_CALLBACK_URL"))
	if callbackURL == "" {
		c.Writef("SSO Callback URL: ")

		callbackURL, err = c.ReadSecret()
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

	if err := c.SettingWriteKey("sso", "callback_url", callbackURL); err != nil {
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

	callbackURL, err := c.SettingReadKey("sso", "callback_url")
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

	parsedUrl, err := url.Parse(callbackURL)
	if err != nil {
		return fmt.Errorf("Error parsing the Callback URL: %s", err)
	}

	// Extract the port and path
	port := parsedUrl.Port()
	path := parsedUrl.Path

	server := &http.Server{Addr: path}
	done := make(chan bool)
	errors := make(chan error)

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		authCodeCallbackHandler(w, r, c, p, done)
	})

	go func() {
		l, err := net.Listen("tcp", fmt.Sprintf(":%s", port))
		if err != nil {
			fmt.Printf("snap: can't listen to port %s: %s\n", port, err)
			errors <- err
		}

		server.Serve(l)
	}()

	select {
	case err := <-errors:
		return err
	default:
		log.Println("Waiting for login... ")
	}

	time.Sleep(1 * time.Second)
	sso.Openbrowser(p.RedirectPath())
	time.Sleep(1 * time.Second)

	// Wait for the user authentication to complete
	<-done

	// Shutdown the HTTP server gracefully
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		fmt.Println("Error shutting down server:", err)
		return err
	}

	return c.OK()
}

func authCodeCallbackHandler(w http.ResponseWriter, r *http.Request, c *stdcli.Context, p structs.SsoProvider, done chan bool) {
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
	file, err := openAuthHTMLFile()
	if err != nil {
		fmt.Fprintf(w, "Could not open the success authentication html page")
		return
	}
	w.Write(file)

	done <- true
}

func openAuthHTMLFile() ([]byte, error) {
	box := packr.NewBox("../../public")
	html, err := box.Find("auth.html")

	if err != nil {
		return nil, err
	}

	return html, nil
}
