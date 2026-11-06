package k8s

import (
	"context"
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
				return nil, errors.WithStack(structs.ErrBadRequest("invalid auth for registry: %s", host))
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
	prefix := p.Engine.RepositoryPrefix()

	if artifactUploadPath(prefix, c.Request().URL.Path) {
		return p.registryUploadProxy(c, prefix)
	}

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

	return registryForward(c, host, fmt.Sprintf("/v2/%s", path), user, pass, prefix)
}

func (p *Provider) registryUploadProxy(c *stdapi.Context, prefix string) error {
	user, pass, err := p.Engine.RepositoryAuth("")
	if err != nil {
		return registryError(err)
	}

	host, _, err := p.Engine.RepositoryHost("")
	if err != nil {
		return registryError(err)
	}

	upstream, local, _ := artifactUploadPrefixes(prefix)

	path := c.Request().URL.Path
	if rest, ok := strings.CutPrefix(path, local+"artifacts-uploads/"); ok {
		path = upstream + rest
	}

	return registryForward(c, host, path, user, pass, prefix)
}

// Artifact Registry continues blob uploads at Locations that name no app: first under
// /artifacts-uploads, then under a pkg image in the /v2 API. Docker only authenticates
// /v2 requests, so the proxy serves the /artifacts-uploads Locations under the pkg path.
func artifactUploadPrefixes(prefix string) (string, string, bool) {
	parts := strings.Split(strings.TrimSuffix(prefix, "/"), "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}

	return fmt.Sprintf("/artifacts-uploads/namespaces/%s/repositories/%s/uploads/", parts[0], parts[1]),
		fmt.Sprintf("/v2/%s/%s/pkg/blobs/uploads/", parts[0], parts[1]), true
}

func artifactUploadPath(prefix, path string) bool {
	_, local, ok := artifactUploadPrefixes(prefix)

	return ok && strings.HasPrefix(path, local) && len(path) > len(local)
}

func artifactUploadLocation(prefix, location string) string {
	upstream, local, ok := artifactUploadPrefixes(prefix)
	if rest, found := strings.CutPrefix(location, upstream); ok && found {
		return local + "artifacts-uploads/" + rest
	}

	return location
}

func registryForward(c *stdapi.Context, host, path, user, pass, prefix string) error {
	u, err := url.Parse(fmt.Sprintf("https://%s", host))
	if err != nil {
		return registryError(err)
	}

	u.Path = path
	u.RawQuery = c.Request().URL.RawQuery

	proxy := httputil.ReverseProxy{
		Director: func(r *http.Request) {
			r.Host = u.Host
			r.URL = u
			r.SetBasicAuth(user, pass)
		},
		ModifyResponse: func(res *http.Response) error {
			l := res.Header.Get("Location")
			if nl := artifactUploadLocation(prefix, l); nl != l {
				res.Header.Set("Location", nl)
			}
			return nil
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
		return errors.WithStack(structs.ErrNotFound("no such registry: %s", server))
	}
	if _, ok := dc.Auths[server]; !ok {
		return errors.WithStack(structs.ErrNotFound("no such registry: %s", server))
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
	s, err := p.Cluster.CoreV1().Secrets(p.Namespace).Get(context.TODO(), secret, am.GetOptions{})
	if ae.IsNotFound(err) {
		return &dockerConfig{}, nil
	}
	if err != nil {
		return nil, errors.WithStack(err)
	}
	if s.Type != ac.SecretTypeDockerConfigJson {
		return nil, errors.WithStack(structs.ErrBadRequest("invalid type for secret: %s", secret))
	}
	data, ok := s.Data[".dockerconfigjson"]
	if !ok {
		return nil, errors.WithStack(structs.ErrBadRequest("invalid data for secret: %s", secret))
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

	s, err := p.Cluster.CoreV1().Secrets(p.Namespace).Get(context.TODO(), secret, am.GetOptions{})
	if ae.IsNotFound(err) {
		_, err := p.Cluster.CoreV1().Secrets(p.Namespace).Create(
			context.TODO(),
			&ac.Secret{
				ObjectMeta: am.ObjectMeta{
					Name: "registries",
					Labels: map[string]string{
						"system": "convox",
						"rack":   p.Name,
					},
				},
				Type: ac.SecretTypeDockerConfigJson,
				Data: sd,
			},
			am.CreateOptions{},
		)
		return errors.WithStack(err)
	}

	s.Data = sd

	_, err = p.Cluster.CoreV1().Secrets(p.Namespace).Update(context.TODO(), s, am.UpdateOptions{})
	if err != nil {
		return errors.WithStack(err)
	}

	return nil
}

func (p *Provider) registryApp(path string) (string, error) {
	app := strings.Split(strings.TrimPrefix(path, p.Engine.RepositoryPrefix()), "/")[0]

	if structs.ReservedAppName(app) {
		return "", errors.WithStack(structs.ErrBadRequest("app name is reserved"))
	}

	if _, err := p.AppGet(app); err != nil {
		return "", err
	}

	return app, nil
}

func registryError(err error) error {
	return stdapi.Errorf(403, `{"errors":[{"message":%q}]}`, err.Error())
}
