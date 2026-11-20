package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/convox/convox/sdk"
	"github.com/convox/stdcli"
	"github.com/gorilla/websocket"
)

const sandboxDefaultAPIURL = "https://console.convox.com"

func init() {
	apiURLFlag := stdcli.StringFlag("api-url", "", "sandbox API URL (default $CONVOX_API_URL, else "+sandboxDefaultAPIURL+")")

	registerWithoutProvider("sandbox proxy", "connect stdin and stdout to a sandbox's ssh server (an ssh ProxyCommand)", SandboxProxy, stdcli.CommandOptions{
		Flags:    []stdcli.Flag{apiURLFlag, stdcli.StringFlag("token", "t", "ssh access token"), stdcli.StringFlag("token-file", "", "file holding the ssh access token")},
		Usage:    "<sandbox-id>",
		Validate: stdcli.Args(1),
	})

	registerWithoutProvider("sandbox ssh", "open an ssh session in a sandbox (needs CONVOX_DEPLOY_KEY)", SandboxSSH, stdcli.CommandOptions{
		Flags:    []stdcli.Flag{apiURLFlag, stdcli.IntFlag("expires", "e", "minutes the access lasts (default 60)")},
		Usage:    "<sandbox-id> [-- command...]",
		Validate: stdcli.ArgsMin(1),
	})

	registerWithoutProvider("sandbox ssh-config", "print an ssh config entry for a sandbox, for scp and VS Code Remote (needs CONVOX_DEPLOY_KEY)", SandboxSSHConfig, stdcli.CommandOptions{
		Flags:    []stdcli.Flag{apiURLFlag, stdcli.IntFlag("expires", "e", "minutes the access lasts (default 1440)")},
		Usage:    "<sandbox-id>",
		Validate: stdcli.Args(1),
	})
}

func sandboxAPIURL(c *stdcli.Context) string {
	if v := c.String("api-url"); v != "" {
		return strings.TrimRight(v, "/")
	}
	if v := os.Getenv("CONVOX_API_URL"); v != "" {
		return strings.TrimRight(v, "/")
	}
	return sandboxDefaultAPIURL
}

// SandboxProxy relays stdin and stdout over the sandbox's ssh tunnel. ssh runs it as a ProxyCommand.
func SandboxProxy(_ sdk.Interface, c *stdcli.Context) error {
	token := c.String("token")
	if f := c.String("token-file"); token == "" && f != "" {
		data, err := os.ReadFile(f)
		if err != nil {
			return err
		}
		token = strings.TrimSpace(string(data))
	}
	if token == "" {
		token = os.Getenv("CONVOX_SANDBOX_SSH_TOKEN")
	}
	if token == "" {
		return fmt.Errorf("--token is required")
	}
	u, err := url.Parse(sandboxAPIURL(c))
	if err != nil {
		return err
	}
	switch u.Scheme {
	case "https":
		u.Scheme = "wss"
	case "http":
		u.Scheme = "ws"
	default:
		return fmt.Errorf("api url must be http or https")
	}
	u.Path = strings.TrimRight(u.Path, "/") + "/apis/v1/sandboxes/" + url.PathEscape(c.Arg(0)) + "/ssh"

	dialer := websocket.Dialer{HandshakeTimeout: 30 * time.Second, Proxy: http.ProxyFromEnvironment}
	ws, res, err := dialer.Dial(u.String(), http.Header{"X-Convox-Ssh-Token": {token}})
	if err != nil {
		if res != nil {
			return sandboxResponseError(res)
		}
		return err
	}
	defer ws.Close()

	done := make(chan struct{})
	go func() {
		defer close(done)
		buf := make([]byte, 32<<10)
		for {
			n, err := c.Reader().Read(buf)
			if n > 0 {
				if werr := ws.WriteMessage(websocket.BinaryMessage, buf[:n]); werr != nil {
					return
				}
			}
			if err != nil {
				_ = ws.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""), time.Now().Add(time.Second))
				return
			}
		}
	}()

	out := c.Writer().Stdout
	for {
		kind, data, err := ws.ReadMessage()
		if err != nil {
			return nil
		}
		if kind != websocket.BinaryMessage {
			continue
		}
		if _, err := out.Write(data); err != nil {
			return nil
		}
	}
}

type sandboxSSHAccess struct {
	Token     string    `json:"token"`
	SandboxID string    `json:"sandbox_id"`
	ExpiresAt time.Time `json:"expires_at"`
	User      string    `json:"user"`
}

func sandboxResponseError(res *http.Response) error {
	data, _ := io.ReadAll(io.LimitReader(res.Body, 64<<10))
	var env struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if json.Unmarshal(data, &env) == nil && env.Error.Message != "" {
		return fmt.Errorf("%s (%s)", env.Error.Message, env.Error.Code)
	}
	return fmt.Errorf("sandbox api answered %d", res.StatusCode)
}

// sandboxKey returns this machine's sandbox ssh key, creating it on first use.
func sandboxKey(c *stdcli.Context) (string, string, error) {
	dir, err := c.SettingDirectory("sandbox-ssh")
	if err != nil {
		return "", "", err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", "", err
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		return "", "", err
	}
	key := filepath.Join(dir, "id_ed25519")
	if _, err := os.Stat(key); err != nil {
		if out, err := c.Execute("ssh-keygen", "-q", "-t", "ed25519", "-N", "", "-C", "convox-sandbox", "-f", key); err != nil {
			return "", "", fmt.Errorf("could not create an ssh key with ssh-keygen: %s", strings.TrimSpace(string(out)))
		}
	}
	return key, dir, nil
}

func sandboxCreateSSHAccess(c *stdcli.Context, id, publicKey string, minutes int) (*sandboxSSHAccess, error) {
	deployKey := os.Getenv("CONVOX_DEPLOY_KEY")
	if deployKey == "" {
		return nil, fmt.Errorf("set CONVOX_DEPLOY_KEY to a deploy key with sandbox access")
	}
	if err := checkSandboxAPIURL(sandboxAPIURL(c)); err != nil {
		return nil, err
	}
	body, _ := json.Marshal(map[string]interface{}{"public_key": publicKey, "expires_in_minutes": minutes})
	req, err := http.NewRequest("POST", sandboxAPIURL(c)+"/apis/v1/sandboxes/"+url.PathEscape(id)+"/ssh-access", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Deploy-Token", deployKey)
	res, err := sandboxHTTPClient().Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, sandboxResponseError(res)
	}
	var access sandboxSSHAccess
	if err := json.NewDecoder(res.Body).Decode(&access); err != nil {
		return nil, err
	}
	if !sandboxIDPattern.MatchString(access.SandboxID) || !sandboxTokenPattern.MatchString(access.Token) || !sandboxUserPattern.MatchString(access.User) {
		return nil, fmt.Errorf("the sandbox api returned an unexpected ssh access")
	}
	return &access, nil
}

var (
	sandboxIDPattern    = regexp.MustCompile(`^[A-Za-z0-9-]{1,64}$`)
	sandboxTokenPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{16,128}$`)
	sandboxUserPattern  = regexp.MustCompile(`^[a-z_][a-z0-9_-]{0,31}$`)
)

// sandboxHTTPClient never follows redirects: the deploy key header would go to wherever a redirect points.
func sandboxHTTPClient() *http.Client {
	return &http.Client{
		Timeout:       60 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
}

// checkSandboxAPIURL refuses to send a deploy key over plain http, except to this machine.
func checkSandboxAPIURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return err
	}
	switch {
	case u.Scheme == "https":
		return nil
	case u.Scheme == "http" && (u.Hostname() == "localhost" || u.Hostname() == "127.0.0.1" || u.Hostname() == "::1"):
		return nil
	}
	return fmt.Errorf("the sandbox api url must use https: %s", raw)
}

func sandboxSSHSetup(c *stdcli.Context, defaultMinutes int) (*sandboxSSHAccess, string, string, string, error) {
	key, dir, err := sandboxKey(c)
	if err != nil {
		return nil, "", "", "", err
	}
	pub, err := os.ReadFile(key + ".pub")
	if err != nil {
		return nil, "", "", "", err
	}
	minutes := c.Int("expires")
	if minutes == 0 {
		minutes = defaultMinutes
	}
	access, err := sandboxCreateSSHAccess(c, c.Arg(0), strings.TrimSpace(string(pub)), minutes)
	if err != nil {
		return nil, "", "", "", err
	}
	self, err := os.Executable()
	if err != nil {
		return nil, "", "", "", err
	}
	tokenFile := filepath.Join(dir, "token-"+access.SandboxID)
	if err := os.WriteFile(tokenFile, []byte(access.Token), 0o600); err != nil {
		return nil, "", "", "", err
	}
	if err := os.Chmod(tokenFile, 0o600); err != nil {
		return nil, "", "", "", err
	}
	proxy := fmt.Sprintf("%s sandbox proxy %s --token-file %s --api-url %s",
		shellQuote(self), access.SandboxID, shellQuote(tokenFile), shellQuote(sandboxAPIURL(c)))
	return access, key, filepath.Join(dir, "known_hosts"), proxy, nil
}

func shellQuote(s string) string {
	if safeShellWord.MatchString(s) {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

var safeShellWord = regexp.MustCompile(`^[A-Za-z0-9_@%+=:,./-]+$`)

// SandboxSSH opens an interactive ssh session (or runs a command) in a sandbox.
func SandboxSSH(_ sdk.Interface, c *stdcli.Context) error {
	access, key, known, proxy, err := sandboxSSHSetup(c, 60)
	if err != nil {
		return err
	}
	args := []string{
		"-i", key,
		"-o", "IdentitiesOnly=yes",
		"-o", "ProxyCommand=" + proxy,
		"-o", "StrictHostKeyChecking=accept-new",
		"-o", "UserKnownHostsFile=" + known,
		fmt.Sprintf("%s@%s", access.User, access.SandboxID),
	}
	args = append(args, c.Args[1:]...)
	return c.Terminal("ssh", args...)
}

// SandboxSSHConfig prints an ssh config entry, so scp, sftp and VS Code Remote reach the sandbox by name.
func SandboxSSHConfig(_ sdk.Interface, c *stdcli.Context) error {
	access, key, known, proxy, err := sandboxSSHSetup(c, 24*60)
	if err != nil {
		return err
	}
	fmt.Fprintf(c.Writer().Stdout, `# convox sandbox %[1]s: access expires %[2]s; run this command again after that.
Host convox-sandbox-%[1]s
  HostName %[1]s
  User %[3]s
  IdentityFile %[4]s
  IdentitiesOnly yes
  ProxyCommand %[5]s
  StrictHostKeyChecking accept-new
  UserKnownHostsFile %[6]s
`, access.SandboxID, access.ExpiresAt.Format(time.RFC3339), access.User, key, proxy, known)
	return nil
}
