package stdapi

import (
	"bytes"
	"fmt"
	"html/template"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/gobuffalo/packr"
	"github.com/pkg/errors"
)

var (
	templateBox     packr.Box
	templateHelpers TemplateHelpers
)

type TemplateHelpers func(c *Context) template.FuncMap

func LoadTemplates(box packr.Box, helpers TemplateHelpers) {
	templateBox = box
	templateHelpers = helpers
}

func TemplateExists(path string) bool {
	return templateBox.Has(fmt.Sprintf("%s.tmpl", path))
}

func RenderTemplate(c *Context, path string, params interface{}) error {
	return RenderTemplatePart(c, path, "main", params)
}

func RenderTemplatePart(c *Context, path, part string, params interface{}) error {
	files := []string{}

	files = append(files, "layout.tmpl")

	parts := strings.Split(filepath.Dir(path), "/")

	for i := range parts {
		files = append(files, filepath.Join(filepath.Join(parts[0:i+1]...), "layout.tmpl"))
	}

	files = append(files, fmt.Sprintf("%s.tmpl", path))

	ts := template.New(part)

	if templateHelpers != nil {
		ts = ts.Funcs(templateHelpers(c))
	}

	for _, f := range files {
		if templateBox.Has(f) {
			if _, err := ts.Parse(templateBox.String(f)); err != nil {
				return errors.WithStack(err)
			}
		}
	}

	var buf bytes.Buffer

	if err := ts.Execute(&buf, params); err != nil {
		return errors.WithStack(err)
	}

	io.Copy(c, &buf)

	return nil
}

func appendIfExists(files []string, path string) []string {
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		files = append(files, path)
	}

	return files
}
