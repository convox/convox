package exoscale

import (
	"fmt"
	"html/template"

	"github.com/convox/convox/pkg/common"
)

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
