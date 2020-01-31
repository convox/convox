package aws

import (
	"fmt"
	"html/template"

	"github.com/convox/convox/pkg/common"
)

func (p *Provider) RenderTemplate(name string, params map[string]interface{}) ([]byte, error) {
	data, err := p.templater.Render(fmt.Sprintf("%s.yml.tmpl", name), params)
	if err != nil {
		return nil, err
	}

	return common.FormatYAML(data)
}

func (p *Provider) templateHelpers() template.FuncMap {
	return template.FuncMap{
		"coalesce": func(ss ...string) string {
			return common.CoalesceString(ss...)
		},
		"safe": func(s string) template.HTML {
			return template.HTML(fmt.Sprintf("%q", s))
		},
		"upper": func(s string) string {
			return common.UpperName(s)
		},
	}
}
