package atom

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html/template"
	"os/exec"
	"sort"
	"strings"
	"time"

	aa "github.com/convox/convox/pkg/atom/pkg/apis/atom/v1"
	av "github.com/convox/convox/pkg/atom/pkg/client/clientset/versioned"
	"github.com/convox/convox/pkg/templater"
	"github.com/gobuffalo/packr"
	"github.com/pkg/errors"
	yaml "gopkg.in/yaml.v2"
	ac "k8s.io/api/core/v1"
	ae "k8s.io/apimachinery/pkg/api/errors"
	am "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
)

var (
	templates = templater.New(packr.NewBox("../atom/templates"), templateHelpers())
)

type Client struct {
	config *rest.Config
	atom   av.Interface
	k8s    kubernetes.Interface
}

func New(cfg *rest.Config) (*Client, error) {
	ac, err := av.NewForConfig(cfg)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	kc, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	c := &Client{
		atom:   ac,
		config: cfg,
		k8s:    kc,
	}

	return c, nil
}

func Apply(namespace, name, release string, template []byte, timeout int32) error {
	params := map[string]interface{}{
		"Name":      name,
		"Namespace": namespace,
		"Release":   release,
		"Template":  base64.StdEncoding.EncodeToString(template),
		"Timeout":   timeout,
		"Version":   time.Now().UTC().UnixNano(),
	}

	if err := exec.Command("kubectl", "get", fmt.Sprintf("ns/%s", namespace)).Run(); err != nil {
		if err := kubectlApplyTemplate("namespace.yml.tmpl", params); err != nil {
			return errors.WithStack(err)
		}

		for {
			if err := exec.Command("kubectl", "get", fmt.Sprintf("ns/%s", namespace)).Run(); err == nil {
				break
			}

			time.Sleep(1 * time.Second)
		}
	}

	data, err := templates.Render("version.yml.tmpl", params)
	if err != nil {
		return errors.WithStack(err)
	}

	if data, err := kubectlCreate(data); err != nil {
		return errors.WithStack(fmt.Errorf("could not create atom version for %s/%s: %s", name, release, strings.TrimSpace(string(data))))
	}

	if err := kubectlApplyTemplate("atom.yml.tmpl", params); err != nil {
		return errors.WithStack(err)
	}

	return nil
}

func (c *Client) Apply(ns, name string, release string, template []byte, timeout int32) error {
	if _, err := c.k8s.CoreV1().Namespaces().Get(ns, am.GetOptions{}); ae.IsNotFound(err) {
		if err := c.createNamespace(ns); err != nil {
			return errors.WithStack(err)
		}
	}

	v, err := c.atom.AtomV1().AtomVersions(ns).Create(&aa.AtomVersion{
		ObjectMeta: am.ObjectMeta{
			Name: fmt.Sprintf("%s-%d", name, time.Now().UTC().UnixNano()),
		},
		Spec: aa.AtomVersionSpec{
			Release:  release,
			Template: template,
		},
	})
	if err != nil {
		return errors.WithStack(err)
	}

	a, err := c.atom.AtomV1().Atoms(ns).Get(name, am.GetOptions{})
	switch {
	case ae.IsNotFound(err):
		a, err = c.atom.AtomV1().Atoms(ns).Create(&aa.Atom{
			ObjectMeta: am.ObjectMeta{
				Name: name,
			},
		})
		if err != nil {
			return errors.WithStack(err)
		}
	case err != nil:
		return errors.WithStack(err)
	default:
		a.Spec.PreviousVersion = a.Spec.CurrentVersion
	}

	a.Spec.CurrentVersion = v.Name
	a.Spec.ProgressDeadlineSeconds = timeout
	a.Started = am.Now()
	a.Status = "Pending"

	if _, err := c.atom.AtomV1().Atoms(ns).Update(a); err != nil {
		return errors.WithStack(err)
	}

	return nil
}

func (c *Client) Cancel(ns, name string) error {
	a, err := c.atom.AtomV1().Atoms(ns).Get(name, am.GetOptions{})
	if err != nil {
		return errors.WithStack(err)
	}

	switch a.Status {
	case "Updating":
	default:
		return errors.WithStack(fmt.Errorf("not currently updating"))
	}

	a.Status = "Cancelled"

	if _, err := c.atom.AtomV1().Atoms(ns).Update(a); err != nil {
		return errors.WithStack(err)
	}

	return nil
}

func (c *Client) Status(ns, name string) (string, string, error) {
	a, err := c.atom.AtomV1().Atoms(ns).Get(name, am.GetOptions{})
	if ae.IsNotFound(err) {
		return "", "", nil
	}
	if err != nil {
		return "", "", errors.WithStack(err)
	}

	release := ""

	if a.Spec.CurrentVersion != "" {
		v, err := c.atom.AtomV1().AtomVersions(ns).Get(a.Spec.CurrentVersion, am.GetOptions{})
		if err != nil {
			return "", "", errors.WithStack(err)
		}

		release = v.Spec.Release
	}

	return string(a.Status), release, nil
}

func (c *Client) apply(a *aa.Atom) error {
	fmt.Printf("ns=atom at=apply atom=\"%s/%s\" version=%q\n", a.Namespace, a.Name, a.Spec.CurrentVersion)

	if a.Spec.CurrentVersion == "" {
		return nil
	}

	ua, err := c.update(a, func(ua *aa.Atom) {
		ua.Started = am.Now()
	})
	if err != nil {
		return err
	}

	av, err := c.atom.AtomV1().AtomVersions(ua.Namespace).Get(ua.Spec.CurrentVersion, am.GetOptions{})
	if err != nil {
		return errors.WithStack(err)
	}

	hash := fmt.Sprintf("%x", sha256.Sum256([]byte(fmt.Sprintf("%s.%s", ua.Namespace, ua.Name))))[0:60]

	if out, err := applyTemplate(av.Spec.Template, fmt.Sprintf("atom=%s", hash)); err != nil {
		fmt.Println(string(av.Spec.Template))
		fmt.Println(string(out))
		return errors.WithStack(err)
	}

	return nil
}

func (c *Client) check(ns, version string) (bool, error) {
	fmt.Printf("ns=atom at=check ns=%q version=%q\n", ns, version)

	if version == "" {
		return true, nil
	}

	cfg := *c.config

	cfg.APIPath = "/apis"
	cfg.NegotiatedSerializer = serializer.WithoutConversionCodecFactory{CodecFactory: scheme.Codecs}

	v, err := c.atom.AtomV1().AtomVersions(ns).Get(version, am.GetOptions{})
	if err != nil {
		return false, errors.WithStack(err)
	}

	cs, err := extractConditions(v.Spec.Template)
	if err != nil {
		return false, errors.WithStack(err)
	}

	for _, c := range cs {
		gv, err := schema.ParseGroupVersion(c.ApiVersion)
		if err != nil {
			return false, errors.WithStack(err)
		}

		cfg.GroupVersion = &gv

		rc, err := rest.RESTClientFor(&cfg)
		if err != nil {
			return false, errors.WithStack(err)
		}

		data, err := rc.Get().Namespace(c.Namespace).Name(c.Name).VersionedParams(&am.GetOptions{}, scheme.ParameterCodec).Resource(fmt.Sprintf("%ss", strings.ToLower(c.Kind))).Do().Raw()
		if err != nil {
			return false, errors.WithStack(err)
		}

		var o struct {
			Status struct {
				Conditions []struct {
					Type   string
					Status string
					Reason string
				}
			}
		}

		if err := json.Unmarshal(data, &o); err != nil {
			return false, errors.WithStack(err)
		}

		css := map[string]string{}
		crs := map[string]string{}

		for _, c := range o.Status.Conditions {
			css[c.Type] = c.Status
			crs[c.Type] = c.Reason
		}

		for k, c := range c.Conditions {
			if c.Status != css[k] {
				return false, nil
			}
			if c.Reason != "" && c.Reason != crs[k] {
				return false, nil
			}
		}
	}

	return true, nil
}

func (c *Client) createNamespace(ns string) error {
	_, err := c.k8s.CoreV1().Namespaces().Create(&ac.Namespace{
		ObjectMeta: am.ObjectMeta{
			Name: ns,
		},
	})
	if err != nil {
		return errors.WithStack(err)
	}

	for {
		if ns, err := c.k8s.CoreV1().Namespaces().Get(ns, am.GetOptions{}); err == nil && ns != nil {
			break
		}

		time.Sleep(1 * time.Second)
	}

	return nil
}

func (c *Client) rollback(a *aa.Atom) error {
	ua, err := c.update(a, func(ua *aa.Atom) {
		ua.Spec.CurrentVersion = a.Spec.PreviousVersion
		ua.Spec.PreviousVersion = ""
	})
	if err != nil {
		return err
	}

	if err := c.apply(ua); err != nil {
		return err
	}

	return nil
}

// use a callback for updates so we can fetch a fresh atom and update
// immediately
func (c *Client) update(a *aa.Atom, fn func(ua *aa.Atom)) (*aa.Atom, error) {
	ua, err := c.atom.AtomV1().Atoms(a.Namespace).Get(a.Name, am.GetOptions{})
	if err != nil {
		return nil, errors.WithStack(err)
	}

	fn(ua)

	fa, err := c.atom.AtomV1().Atoms(a.Namespace).Update(ua)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return fa, nil
}

func applyLabels(data []byte, labels map[string]string) ([]byte, error) {
	var v map[string]interface{}

	if err := yaml.Unmarshal(data, &v); err != nil {
		return nil, errors.WithStack(err)
	}

	if len(v) == 0 {
		return data, nil
	}

	switch t := v["metadata"].(type) {
	case nil:
		v["metadata"] = map[string]interface{}{"labels": labels}
	case map[interface{}]interface{}:
		switch u := t["labels"].(type) {
		case nil:
			t["labels"] = labels
			v["metadata"] = t
		case map[interface{}]interface{}:
			for k, v := range labels {
				u[k] = v
			}
			t["labels"] = u
			v["metadata"] = t
		default:
			return nil, errors.WithStack(fmt.Errorf("unknown labels type: %T", u))
		}
	default:
		return nil, errors.WithStack(fmt.Errorf("unknown metadata type: %T", t))
	}

	pd, err := yaml.Marshal(v)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return pd, nil
}

func applyTemplate(data []byte, filter string) ([]byte, error) {
	rs, err := templateResources(filter)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	labels := parseLabels(filter)

	parts := bytes.Split(data, []byte("---\n"))

	for i := range parts {
		dp, err := applyLabels(parts[i], labels)
		if err != nil {
			return nil, errors.WithStack(err)
		}

		parts[i] = dp
	}

	data = bytes.Join(parts, []byte("---\n"))

	args := []string{"--prune", "-l", filter}

	for _, r := range rs {
		args = append(args, "--prune-whitelist", r)
	}

	out, err := kubectlApply(data, args...)
	if err != nil {
		if !strings.Contains(string(out), "is immutable") {
			return out, errors.WithStack(err)
		}

		out, err := kubectlApply(data, "--force")
		if err != nil {
			return out, errors.WithStack(err)
		}
	}

	return out, nil
}

func extractConditions(data []byte) ([]aa.AtomCondition, error) {
	cs := []aa.AtomCondition{}

	parts := bytes.Split(data, []byte("---\n"))

	for _, p := range parts {
		var o struct {
			ApiVersion string `yaml:"apiVersion"`
			Kind       string
			Metadata   struct {
				Annotations map[string]string
				Name        string
				Namespace   string
			}
		}

		if err := yaml.Unmarshal(p, &o); err != nil {
			return nil, errors.WithStack(err)
		}

		if ac, ok := o.Metadata.Annotations["atom.conditions"]; ok {
			acps := strings.Split(ac, ",")

			acs := map[string]aa.AtomConditionMatch{}

			for _, acp := range acps {
				if acpps := strings.SplitN(acp, "=", 2); len(acpps) == 2 {
					if vps := strings.SplitN(acpps[1], "/", 2); len(vps) == 2 {
						acs[acpps[0]] = aa.AtomConditionMatch{Status: vps[0], Reason: vps[1]}
					} else {
						acs[acpps[0]] = aa.AtomConditionMatch{Status: vps[0]}
					}
				}
			}

			cs = append(cs, aa.AtomCondition{
				ApiVersion: o.ApiVersion,
				Conditions: acs,
				Kind:       o.Kind,
				Name:       o.Metadata.Name,
				Namespace:  o.Metadata.Namespace,
			})
		}
	}

	return cs, nil
}

func kubectlApply(data []byte, args ...string) ([]byte, error) {
	ka := append([]string{"apply", "-f", "-"}, args...)

	cmd := exec.Command("kubectl", ka...)

	cmd.Stdin = bytes.NewReader(data)

	return cmd.CombinedOutput()
}

func kubectlCreate(data []byte, args ...string) ([]byte, error) {
	ka := append([]string{"create", "-f", "-"}, args...)

	cmd := exec.Command("kubectl", ka...)

	cmd.Stdin = bytes.NewReader(data)

	return cmd.CombinedOutput()
}

func kubectlApplyTemplate(template string, params map[string]interface{}) error {
	data, err := templates.Render(template, params)
	if err != nil {
		return errors.WithStack(err)
	}

	if out, err := kubectlApply(data); err != nil {
		return errors.WithStack(errors.New(strings.TrimSpace(string(out))))
	}

	return nil
}

func parseLabels(labels string) map[string]string {
	ls := map[string]string{}

	for _, part := range strings.Split(labels, ",") {
		ps := strings.SplitN(strings.TrimSpace(part), "=", 2)
		if len(ps) == 2 {
			ls[ps[0]] = ps[1]
		}
	}

	return ls
}

func templateResources(filter string) ([]string, error) {
	data, err := exec.Command("kubectl", "api-resources", "--verbs=list", "--namespaced", "-o", "name").CombinedOutput()
	if err != nil {
		return []string{}, nil
	}

	ars := strings.Split(strings.TrimSpace(string(data)), "\n")

	rsh := map[string]bool{}

	data, err = exec.Command("kubectl", "get", "-l", filter, "--all-namespaces", "-o", "json", strings.Join(ars, ",")).CombinedOutput()
	if err != nil {
		return []string{}, nil
	}

	if strings.TrimSpace(string(data)) == "" {
		return []string{}, nil
	}

	var res struct {
		Items []struct {
			ApiVersion string `json:"apiVersion"`
			Kind       string `json:"kind"`
		}
	}

	if err := json.Unmarshal(data, &res); err != nil {
		return nil, errors.WithStack(err)
	}

	for _, i := range res.Items {
		av := i.ApiVersion

		if !strings.Contains(av, "/") {
			av = fmt.Sprintf("core/%s", av)
		}

		rsh[fmt.Sprintf("%s/%s", av, i.Kind)] = true
	}

	rs := []string{}

	for r := range rsh {
		rs = append(rs, r)
	}

	sort.Strings(rs)

	return rs, nil
}

func templateHelpers() map[string]interface{} {
	return map[string]interface{}{
		"safe": func(s string) template.HTML {
			return template.HTML(fmt.Sprintf("%q", s))
		},
	}
}
