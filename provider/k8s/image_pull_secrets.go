package k8s

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/convox/convox/pkg/manifest"
	"github.com/pkg/errors"
	ac "k8s.io/api/core/v1"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const maxSecretNameLen = 253

var registrySlugReplacer = strings.NewReplacer(".", "-", ":", "-", "/", "-")

// renderImagePullSecrets emits a Kubernetes Secret of kind
// kubernetes.io/dockerconfigjson for each entry in service.ImagePullSecrets.
// It returns the serialized YAML blocks (appended to the atom apply manifest
// so prune-on-remove works automatically via atom=<hash> labelling) and the
// generated Secret names (added to the Pod spec's imagePullSecrets list
// alongside docker-hub-authentication).
//
// Passwords sourced from passwordEnv are resolved via envResolver, which must
// expose the already-merged service-scoped environment. A missing env var
// returns a clear error naming the variable and the service.
func renderImagePullSecrets(app, namespace string, service *manifest.Service, envResolver func(string) (string, bool)) ([][]byte, []string, error) {
	if service == nil || len(service.ImagePullSecrets) == 0 {
		return nil, nil, nil
	}

	blocks := make([][]byte, 0, len(service.ImagePullSecrets))
	names := make([]string, 0, len(service.ImagePullSecrets))

	for i, sec := range service.ImagePullSecrets {
		password := sec.Password
		if sec.PasswordEnv != "" {
			v, ok := envResolver(sec.PasswordEnv)
			if !ok || v == "" {
				return nil, nil, fmt.Errorf("service %s imagePullSecrets[%d]: env var %s is not set; run 'convox env set %s=<value> -a %s' and retry the promote", service.Name, i, sec.PasswordEnv, sec.PasswordEnv, app)
			}
			password = v
		}

		payload, err := buildDockerConfigJSON(sec.Registry, sec.Username, password)
		if err != nil {
			return nil, nil, errors.WithStack(err)
		}

		name := imagePullSecretName(app, service.Name, sec.Registry)

		secretObj := &ac.Secret{
			TypeMeta: am.TypeMeta{
				Kind:       "Secret",
				APIVersion: "v1",
			},
			ObjectMeta: am.ObjectMeta{
				Name:      name,
				Namespace: namespace,
				Labels: map[string]string{
					"system":  "convox",
					"app":     app,
					"service": service.Name,
					"type":    "image-pull",
				},
			},
			Type: ac.SecretTypeDockerConfigJson,
			Data: map[string][]byte{
				ac.DockerConfigJsonKey: payload,
			},
		}

		y, err := SerializeK8sObjToYaml(secretObj)
		if err != nil {
			return nil, nil, errors.WithStack(err)
		}

		blocks = append(blocks, y)
		names = append(names, name)
	}

	return blocks, names, nil
}

// imagePullSecretName generates a deterministic K8s Secret name of the form
// `convox-<app>-<service>-pull-<registry-slug>`, where <registry-slug> is the
// registry host with `.`, `:`, and `/` replaced by `-`. Enforces the K8s
// 253-char name limit with a sha256 suffix if the full name overflows.
func imagePullSecretName(app, service, registry string) string {
	slug := registrySlug(registry)
	full := fmt.Sprintf("convox-%s-%s-pull-%s", app, service, slug)
	if len(full) <= maxSecretNameLen {
		return full
	}
	h := sha256.Sum256([]byte(full))
	suffix := fmt.Sprintf("-%x", h[:4])
	truncated := strings.TrimRight(full[:maxSecretNameLen-len(suffix)], "-")
	return truncated + suffix
}

func registrySlug(registry string) string {
	return strings.ToLower(registrySlugReplacer.Replace(registry))
}

// imagePullSecretNames returns just the Secret names that
// renderImagePullSecrets would have produced. Used by code paths that need
// the Pod-spec references (timer render, convox run) but where the Secrets
// themselves were already emitted by releaseTemplateServices.
func imagePullSecretNames(app, service string, secrets []manifest.ServiceImagePullSecret) []string {
	if len(secrets) == 0 {
		return nil
	}
	names := make([]string, 0, len(secrets))
	for _, sec := range secrets {
		names = append(names, imagePullSecretName(app, service, sec.Registry))
	}
	return names
}

// buildDockerConfigJSON produces the bytes of a .dockerconfigjson payload for
// a single registry. json.Marshal escapes the credential strings correctly
// (passwords containing `"`, `\\`, etc. round-trip safely, unlike the
// fmt.Sprintf form used by ensureDockerHubSecret for the rack-wide auth).
func buildDockerConfigJSON(registry, username, password string) ([]byte, error) {
	auth := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", username, password)))
	cfg := map[string]interface{}{
		"auths": map[string]interface{}{
			registry: map[string]string{
				"auth": auth,
			},
		},
	}
	return json.Marshal(cfg)
}
