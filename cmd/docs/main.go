package main

import (
	"fmt"
	"html/template"
	"os"
	"strings"

	"github.com/convox/convox/pkg/docs"
	"github.com/convox/stdapi"
	"github.com/gobuffalo/packr"
	"github.com/pkg/errors"
)

var categorySlugs = []string{
	"introduction",
	"application",
	"development",
	"deployment",
	"management",
	"use-cases",
	"console",
	"enterprise",
	"example-apps",
	"reference",
	"external-services",
	"migration",
	"gen1",
	"help",
}

var (
	documents *docs.Documents
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	s := stdapi.New("docs", "docs.convox")

	if err := loadDocuments(); err != nil {
		return err
	}

	s.Router.Static("/assets", packr.NewBox("./public/assets"))
	s.Router.Static("/images", packr.NewBox("./public/images"))

	s.Route("GET", "/", index)
	s.Route("GET", "/toc/{slug:.*}", toc)
	s.Route("GET", "/{slug:.*}", doc)

	stdapi.LoadTemplates(packr.NewBox("./templates"), helpers)

	if err := s.Listen("https", ":3000"); err != nil {
		return err
	}

	return nil
}

func helpers(c *stdapi.Context) template.FuncMap {
	return template.FuncMap{
		"env": func(s string) string {
			return os.Getenv(s)
		},
		"expand": func(slug, active string) bool {
			sparts := strings.Split(slug, "/")
			if len(sparts) < 2 {
				return true
			}
			if strings.HasPrefix(active, slug) {
				return true
			}
			if slug == active {
				return true
			}
			return false
		},
		"inc": func(i int) int {
			return i + 1
		},
		"indent": func(i int) int {
			return i*16 + 10
		},
		"join": func(sep string, ss []string) template.HTML {
			return template.HTML(strings.Join(ss, sep))
		},
		"mul": func(x, y int) int {
			return x * y
		},
		"params": func(args ...interface{}) (map[interface{}]interface{}, error) {
			if len(args)%2 != 0 {
				return nil, errors.WithStack(fmt.Errorf("must have an even number of args"))
			}
			p := map[interface{}]interface{}{}
			for i := 0; i < len(args); i += 2 {
				p[args[i]] = args[i+1]
			}
			return p, nil
		},
		"slugid": func(slug string) string {
			return strings.ToLower(strings.ReplaceAll(slug, "/", "-"))
		},
	}
}

func index(c *stdapi.Context) error {
	return c.Redirect(302, "/getting-started/introduction")
}

func loadDocuments() error {
	ds, err := docs.Load(packr.NewBox("../../docs"))
	if err != nil {
		return err
	}

	documents = ds

	if err := ds.UploadIndex(); err != nil {
		return err
	}

	return nil
}

func doc(c *stdapi.Context) error {
	params := map[string]interface{}{
		"Documents": documents,
		"Slug":      "",
	}

	d, ok := documents.Find(c.Var("slug"))
	if !ok {
		c.Response().WriteHeader(404)
		return c.RenderTemplate("404", params)
	}

	params["Body"] = template.HTML(d.Body)
	params["Breadcrumbs"] = documents.Breadcrumbs(d.Slug)
	params["Category"] = d.Category()
	params["Path"] = d.Path
	params["Slug"] = d.Slug

	if c.Ajax() {
		return c.RenderTemplate("doc", params)
	}

	return c.RenderTemplate("page", params)
}

func toc(c *stdapi.Context) error {
	params := map[string]interface{}{
		"Documents": documents,
		"Slug":      c.Var("slug"),
	}

	return c.RenderTemplate("toc", params)
}
