package common

import (
	"bytes"
	"html"
	"regexp"

	yaml "gopkg.in/yaml.v2"
)

var (
	yamlSplitter = regexp.MustCompile(`(?m)^\s*---\s*$`)
)

func FormatYAML(data []byte) ([]byte, error) {

	// Unescape HTML entities first
	data = []byte(html.UnescapeString(string(data)))

	ps := yamlSplitter.Split(string(data), -1)
	bs := make([][]byte, len(ps))

	for i, p := range ps {
		var v interface{}

		if err := yaml.Unmarshal([]byte(p), &v); err != nil {
			return nil, err
		}

		data, err := yaml.Marshal(v)
		if err != nil {
			return nil, err
		}

		bs[i] = data
	}

	return bytes.Join(bs, []byte("---\n")), nil
}
