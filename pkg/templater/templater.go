package templater

import (
	"bytes"
	"embed"
	"html/template"
)

// Templater holds the embedded filesystem and template helpers.
type Templater struct {
	fs      embed.FS
	helpers template.FuncMap
}

// New creates a new Templater with the given embed.FS and helpers.
func New(fs embed.FS, helpers template.FuncMap) *Templater {
	return &Templater{
		fs:      fs,
		helpers: helpers,
	}
}

// Render renders the template with the given name and parameters.
func (t *Templater) Render(name string, params interface{}) ([]byte, error) {
	ts := template.New("").Funcs(t.helpers)

	tdata, err := t.fs.ReadFile(name)
	if err != nil {
		return nil, err
	}

	if _, err := ts.Parse(string(tdata)); err != nil {
		return nil, err
	}

	var buf bytes.Buffer

	if err := ts.Execute(&buf, params); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}
