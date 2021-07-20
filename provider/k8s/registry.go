package k8s

import (
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"github.com/convox/convox/pkg/common"
	"github.com/convox/convox/pkg/structs"
	"github.com/convox/stdapi"
	"github.com/pkg/errors"
	ac "k8s.io/api/core/v1"
	ae "k8s.io/apimachinery/pkg/api/errors"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (p *Provider) RegistryAdd(server, username, password string) (*structs.Registry, error) {
	dc, err := p.dockerConfigLoad("registries")
	if err != nil {
		return nil, errors.WithStack(err)
	}

	if dc.Auths == nil {
		dc.Auths = map[string]dockerConfigAuth{}
	}

	dc.Auths[server] = dockerConfigAuth{
		Auth: base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", username, password))),
	}

	if err := p.dockerConfigSave("registries", dc); err != nil {
		return nil, errors.WithStack(err)
	}

	r := &structs.Registry{
		Server:   server,
		Username: username,
		Password: password,
	}

	return r, nil
}

// override this function to provider infrastructure-specific authentication, such as
// token swapping for ecr
func (p *Provider) RegistryAuth(host, username, password string) (string, string, error) {
	return username, password, nil
}

func (p *Provider) RegistryList() (structs.Registries, error) {
	dc, err := p.dockerConfigLoad("registries")
	if err != nil {
		return nil, errors.WithStack(err)
	}

	rs := structs.Registries{}

	if dc.Auths != nil {
		for host, auth := range dc.Auths {
			data, err := base64.StdEncoding.DecodeString(auth.Auth)
			if err != nil {
				return nil, errors.WithStack(err)
			}

			parts := strings.SplitN(string(data), ":", 2)
			if len(parts) != 2 {
				return nil, errors.WithStack(fmt.Errorf("invalid auth for registry: %s", host))
			}

			rs = append(rs, structs.Registry{
				Server:   host,
				Username: parts[0],
				Password: parts[1],
			})
		}
	}

	return rs, nil
}

func (p *Provider) RegistryProxy(c *stdapi.Context) error {
	path := c.Var("path")

	if path == "" {
		c.Response().Header().Add("Docker-Distribution-Api-Version", "registry/2.0")
		return nil
	}

	app, err := p.registryApp(path)
	if err != nil {
		return registryError(err)
	}

	user, pass, err := p.Engine.RepositoryAuth(app)
	if err != nil {
		return registryError(err)
	}

	host, _, err := p.Engine.RepositoryHost(app)
	if err != nil {
		return registryError(err)
	}

	u, err := url.Parse(fmt.Sprintf("https://%s", host))
	if err != nil {
		registryError(err)
	}

	u.Path = fmt.Sprintf("/v2/%s", path)
	u.RawQuery = c.Request().URL.RawQuery

	proxy := httputil.ReverseProxy{
		Director: func(r *http.Request) {
			r.Host = u.Host
			r.URL = u
			r.SetBasicAuth(user, pass)
		},
	}

	t := common.NewDefaultTransport()
	t.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}

	proxy.Transport = t

	proxy.ServeHTTP(c.Response(), c.Request())

	return nil
}

func (p *Provider) RegistryRemove(server string) error {
	dc, err := p.dockerConfigLoad("registries")
	if err != nil {
		return errors.WithStack(err)
	}
	if dc.Auths == nil {
		return errors.WithStack(fmt.Errorf("no such registry: %s", server))
	}
	if _, ok := dc.Auths[server]; !ok {
		return errors.WithStack(fmt.Errorf("no such registry: %s", server))
	}

	delete(dc.Auths, server)

	if err := p.dockerConfigSave("registries", dc); err != nil {
		return errors.WithStack(err)
	}

	return nil
}

type dockerConfig struct {
	Auths map[string]dockerConfigAuth `json:"auths"`
}

type dockerConfigAuth struct {
	Auth string `json:"auth"`
}

func (p *Provider) dockerConfigLoad(secret string) (*dockerConfig, error) {
	s, err := p.Cluster.CoreV1().Secrets(p.Namespace).Get(secret, am.GetOptions{})
	if ae.IsNotFound(err) {
		return &dockerConfig{}, nil
	}
	if err != nil {
		return nil, errors.WithStack(err)
	}
	if s.Type != ac.SecretTypeDockerConfigJson {
		return nil, errors.WithStack(fmt.Errorf("invalid type for secret: %s", secret))
	}
	data, ok := s.Data[".dockerconfigjson"]
	if !ok {
		return nil, errors.WithStack(fmt.Errorf("invalid data for secret: %s", secret))
	}

	var dc dockerConfig

	if err := json.Unmarshal(data, &dc); err != nil {
		return nil, errors.WithStack(err)
	}

	return &dc, nil
}

func (p *Provider) dockerConfigSave(secret string, dc *dockerConfig) error {
	data, err := json.Marshal(dc)
	if err != nil {
		return errors.WithStack(err)
	}

	sd := map[string][]byte{
		".dockerconfigjson": data,
	}

	s, err := p.Cluster.CoreV1().Secrets(p.Namespace).Get(secret, am.GetOptions{})
	if ae.IsNotFound(err) {
		_, err := p.Cluster.CoreV1().Secrets(p.Namespace).Create(&ac.Secret{
			ObjectMeta: am.ObjectMeta{
				Name: "registries",
				Labels: map[string]string{
					"system": "convox",
					"rack":   p.Name,
				},
			},
			Type: ac.SecretTypeDockerConfigJson,
			Data: sd,
		})
		return errors.WithStack(err)
	}

	s.Data = sd

	_, err = p.Cluster.CoreV1().Secrets(p.Namespace).Update(s)
	if err != nil {
		return errors.WithStack(err)
	}

	return nil
}

func (p *Provider) registryApp(path string) (string, error) {
	app := strings.Split(strings.TrimPrefix(path, p.Engine.RepositoryPrefix()), "/")[0]

	if _, err := p.AppGet(app); err != nil {
		return "", err
	}

	return app, nil
}

func registryError(err error) error {
	return stdapi.Errorf(403, fmt.Sprintf(`{"errors":[{"message":%q}]}`, err.Error()))
}
