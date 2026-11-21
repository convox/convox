package k8s

import (
	"context"
	"fmt"
	"html"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/convox/convox/pkg/manifest"
	"github.com/convox/convox/pkg/structs"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const nginxAnnotationPrefix = "nginx.ingress.kubernetes.io/"

var routerRejectedDirectives = []string{"subs_filter"}

var (
	convoxNginxImage   = regexp.MustCompile(`(?:^|/)convox/ingress-nginx(?:[:@]|$)`)
	upstreamNginxImage = regexp.MustCompile(`(?:^|/)ingress-nginx/controller:v(\d+)\.(\d+)\.`)
)

type routerSnippet struct {
	service string
	key     string
	value   string
}

func (p *Provider) validateRouterSnippets(m *manifest.Manifest) error {
	snippets := p.routerSnippets(m)
	messages := []string{}

	if p.RouterUserSnippetsDisable {
		for _, s := range snippets {
			messages = append(messages, fmt.Sprintf("service %s: ingress annotation %s is not allowed while router_user_snippets_disable=true", s.service, s.key))
		}
		return snippetError(messages)
	}

	if p.RouterType == "contour" {
		return nil
	}

	for _, s := range snippets {
		if d := routerRejectedDirective(html.UnescapeString(s.value)); d != "" {
			messages = append(messages, fmt.Sprintf("service %s: ingress annotation %s uses %s, which this rack's router does not support", s.service, s.key, d))
		}
	}

	if len(messages) == 0 || !p.routerLacksSubsFilter() {
		return nil
	}

	return snippetError(messages)
}

func (p *Provider) routerSnippets(m *manifest.Manifest) []routerSnippet {
	reserved := p.reservedNginxAnnotations()
	snippets := []routerSnippet{}

	ss := append(m.Services.Routable().External(), m.Services.Routable().InternalRouter()...)

	for i := range ss {
		s := &ss[i]

		for k, v := range s.IngressAnnotationsMap() {
			key := strings.ToLower(strings.TrimSpace(k))
			if reserved[k] || !strings.HasPrefix(key, nginxAnnotationPrefix) || !strings.HasSuffix(key, "-snippet") {
				continue
			}
			snippets = append(snippets, routerSnippet{service: s.Name, key: strings.TrimSpace(k), value: v})
		}
	}

	return snippets
}

func snippetError(messages []string) error {
	if len(messages) == 0 {
		return nil
	}

	sort.Strings(messages)

	return structs.ErrBadRequest("%s", strings.Join(messages, "\n"))
}

func (p *Provider) routerLacksSubsFilter() bool {
	d, err := p.Cluster.AppsV1().Deployments(p.Namespace).Get(context.TODO(), "ingress-nginx", am.GetOptions{})
	if err != nil {
		return false
	}

	for i := range d.Spec.Template.Spec.Containers {
		if c := &d.Spec.Template.Spec.Containers[i]; c.Name == "system" {
			return routerImageLacksSubsFilter(c.Image)
		}
	}

	return false
}

func routerRejectedDirective(snippet string) string {
	statement, token := true, true

	for i := 0; i < len(snippet); i++ {
		c := snippet[i]

		switch {
		case c == ';' || c == '{' || c == '}' || c == '\n':
			statement, token = true, true
		case c == ' ' || c == '\t' || c == '\r':
			token = true
		case token && c == '#':
			n := strings.IndexByte(snippet[i:], '\n')
			if n < 0 {
				return ""
			}
			i += n - 1
		case token && (c == '"' || c == '\''):
			for i++; i < len(snippet) && snippet[i] != c; i++ {
				if snippet[i] == '\\' {
					i++
				}
			}
			statement, token = false, false
		default:
			if statement {
				for _, d := range routerRejectedDirectives {
					if strings.HasPrefix(snippet[i:], d) {
						return d
					}
				}
			}
			if c == '\\' {
				i++
			}
			statement, token = false, false
		}
	}

	return ""
}

func routerImageLacksSubsFilter(image string) bool {
	if convoxNginxImage.MatchString(image) {
		return true
	}

	m := upstreamNginxImage.FindStringSubmatch(image)
	if m == nil {
		return false
	}

	major, err := strconv.Atoi(m[1])
	if err != nil {
		return false
	}

	minor, err := strconv.Atoi(m[2])
	if err != nil {
		return false
	}

	return major > 1 || (major == 1 && minor >= 13)
}
