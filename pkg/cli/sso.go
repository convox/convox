package cli

import (
	"fmt"
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
			stdcli.StringFlag("redirect_uri", "r", "redirect_uri"),
		},
		Validate: stdcli.ArgsMax(5),
	})

	registerWithoutProvider("sso login", "authenticate with a console using sso", SsoLogin, stdcli.CommandOptions{})
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

	redirectURI := coalesce(c.String("redirect_uri"), os.Getenv("SSO_REDIRECT_URI"))
	if redirectURI == "" {
		c.Writef("SSO Redirect URI: ")

		redirectURI, err = c.ReadSecret()
		if err != nil {
			return err
		}

		c.Writef("\n")
	}

	if err := c.SettingWrite("provider", provider); err != nil {
		return err
	}

	if err := c.SettingWrite("client_id", clientID); err != nil {
		return err
	}

	if err := c.SettingWrite("client_secret", clientSecret); err != nil {
		return err
	}

	if err := c.SettingWrite("issuer", issuer); err != nil {
		return err
	}

	if err := c.SettingWrite("redirect_uri", redirectURI); err != nil {
		return err
	}

	return c.OK()
}

func SsoLogin(rack sdk.Interface, c *stdcli.Context) error {
	provider, err := c.SettingRead("provider")
	if err != nil {
		return err
	}

	clientID, err := c.SettingRead("client_id")
	if err != nil {
		return err
	}

	clientSecret, err := c.SettingRead("client_secret")
	if err != nil {
		return err
	}

	issuer, err := c.SettingRead("issuer")
	if err != nil {
		return err
	}

	redirectURI, err := c.SettingRead("redirect_uri")
	if err != nil {
		return err
	}

	nonce, _ := sso.GenerateNonce()

	p, err := sso.Initialize(provider, structs.SsoProviderOptions{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Issuer:       issuer,
		RedirectURL:  redirectURI,
		Scope:        "openid profile email",
		State:        sso.GenerateState(),
		Nonce:        nonce,
	})

	if err != nil {
		return err
	}

	server := &http.Server{Addr: "/authorization-code/callback"}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		authCodeCallbackHandler(w, r, server, p)
	})

	port := fmt.Sprintf(":%s", "8080")
	l, err := net.Listen("tcp", port)
	if err != nil {
		fmt.Printf("snap: can't listen to port %s: %s\n", port, err)
		os.Exit(1)
	}

	log.Println("You will now be taken to your browser for authentication")

	time.Sleep(1 * time.Second)
	fmt.Println(p.RedirectPath())
	sso.Openbrowser(p.RedirectPath())
	time.Sleep(1 * time.Second)

	server.Serve(l)

	return c.OK()
}

func authCodeCallbackHandler(w http.ResponseWriter, r *http.Request, server *http.Server, p structs.SsoProvider) {
	// Check the state that was returned in the query string is the same as the above state
	if r.URL.Query().Get("state") != p.Opts().State {
		fmt.Fprintln(w, "The state was not as expected")
		return
	}

	// Make sure the code was provided
	if r.URL.Query().Get("code") == "" {
		fmt.Fprintln(w, "The code was not returned or is not accessible")
		return
	}

	exchange := p.ExchangeCode(r.URL.Query().Get("code"), r)
	if exchange.Error != "" {
		fmt.Println(exchange.Error)
		fmt.Println(exchange.ErrorDescription)
		return
	}

	// session, err := sessionStore.Get(r, "okta-hosted-login-session-store")
	// if err != nil {
	// 	http.Error(w, err.Error(), http.StatusInternalServerError)
	// }

	verificationError := p.VerifyToken(exchange.IdToken)

	if verificationError != nil {
		fmt.Println(verificationError)
	}

	// if verificationError == nil {
	// 	session.Values["id_token"] = exchange.IdToken
	// 	session.Values["access_token"] = exchange.AccessToken

	// 	session.Save(r, w)
	// }

	// show succes page
	msg := "<p><strong>Success!</strong></p>"
	msg = msg + "<p>You are authenticated and can now return to the CLI.</p>"
	fmt.Fprintf(w, msg)

	// close the HTTP server
	cleanup(server)
}

func cleanup(server *http.Server) {
	go server.Close()
}
