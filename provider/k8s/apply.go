package k8s

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"

	"github.com/pkg/errors"

	yaml "gopkg.in/yaml.v2"
)

func (p *Provider) Apply(namespace, name, version string, data []byte, labels string, timeout int32) error {
	ldata, err := ApplyLabels(data, labels)
	if err != nil {
		return errors.WithStack(err)
	}

	return p.Atom.Apply(namespace, name, version, ldata, timeout)
}

func Apply(data []byte, args ...string) error {
	ka := append([]string{"apply", "-f", "-"}, args...)

	cmd := exec.Command("kubectl", ka...)

	cmd.Stdin = bytes.NewReader(data)

	out, err := cmd.CombinedOutput()
	if err != nil {
		return errors.WithStack(errors.New(strings.TrimSpace(string(out))))
	}

	return nil
}

func Delete(data []byte, args ...string) error {
	ka := append([]string{"delete", "-f", "-"}, args...)

	cmd := exec.Command("kubectl", ka...)

	cmd.Stdin = bytes.NewReader(data)

	out, err := cmd.CombinedOutput()
	if err != nil {
		return errors.WithStack(errors.New(strings.TrimSpace(string(out))))
	}

	return nil
}

func ApplyLabels(data []byte, labels string) ([]byte, error) {
	ls := parseLabels(labels)

	parts := bytes.Split(data, []byte("---\n"))

	for i := range parts {
		dp, err := applyLabels(parts[i], ls)
		if err != nil {
			return nil, errors.WithStack(err)
		}

		parts[i] = dp
	}

	return bytes.Join(parts, []byte("---\n")), nil
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
