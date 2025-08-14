package docs

import (
	"bytes"
	"embed"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/algolia/algoliasearch-client-go/algoliasearch"
	"github.com/russross/blackfriday"
	"gopkg.in/yaml.v2"
)

var (
	categories = Categories{
		Category{"getting-started", "Getting Started"},
		Category{"tutorials", "Tutorials"},
		Category{"installation", "Installation"},
		Category{"configuration", "Configuration"},
		Category{"development", "Development"},
		Category{"deployment", "Deployment"},
		Category{"management", "Management"},
		Category{"reference", "Reference"},
		Category{"integrations", "Integrations"},
		Category{"help", "Help"},
	}
)

var (
	reDocument    = regexp.MustCompile(`(?ms)(---(.*?)---)?(.*)$`)
	reLink        = regexp.MustCompile(`href="([^"]+)"`)
	reMarkdownDiv = regexp.MustCompile(`(?ms)(<div.*?markdown="1".*?>(.*?)</div>)`)
	reTag         = regexp.MustCompile(`(?ms)(<[\w]+.*?>(.*?)</[\w]+>)`)
	reTitle       = regexp.MustCompile(`<h1.*?>(.*?)</h1>`)
)

type Category struct {
	Slug string
	Name string
}

type Categories []Category

type Document struct {
	Body        []byte
	Description string
	Order       int
	Path        string
	Slug        string
	Tags        []string
	Title       string
}

type Documents struct {
	documents []Document
}

func Load(docFs embed.FS, dir string) (*Documents, error) {
	ds := &Documents{
		documents: []Document{},
	}

	entries, err := docFs.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			// Recursively walk subdirectories
			subDs, err := Load(docFs, dir+"/"+entry.Name())
			if err != nil {
				return nil, err
			}
			ds.documents = append(ds.documents, subDs.documents...)
			continue
		}

		path := dir + "/" + entry.Name()
		data, err := docFs.ReadFile(path)
		if err != nil {
			return nil, err
		}

		d, err := parseDocument(path, data)
		if err != nil {
			return nil, err
		}

		ds.Add(*d)
	}

	return ds, nil
}

func (cs Categories) Find(slug string) Category {
	for _, c := range cs {
		if c.Slug == slug {
			return c
		}
	}
	return Category{}
}

func (ds *Documents) Add(d Document) error {
	ds.documents = append(ds.documents, d)
	return nil
}

func (ds *Documents) Categories() Categories {
	return categories
}

func (ds *Documents) Children(slug string) []Document {
	docs := []Document{}

	for _, d := range ds.documents {
		if strings.HasPrefix(d.Slug, fmt.Sprintf("%s/", slug)) && level(d.Slug) == level(slug)+1 {
			docs = append(docs, d)
		}
	}

	sort.Slice(docs, func(i, j int) bool {
		if docs[i].Order == docs[j].Order {
			return strings.ToLower(docs[i].Title) < strings.ToLower(docs[j].Title)
		}
		return docs[i].Order < docs[j].Order
	})

	return docs
}

func (ds *Documents) Find(slug string) (*Document, bool) {
	for _, d := range ds.documents {
		if d.Slug == slug {
			return &d, true
		}
	}

	return nil, false
}

func (ds *Documents) UploadIndex() error {
	if os.Getenv("ALGOLIA_APP") == "" {
		return nil
	}

	algolia := algoliasearch.NewClient(os.Getenv("ALGOLIA_APP"), os.Getenv("ALGOLIA_KEY_ADMIN")).InitIndex(os.Getenv("ALGOLIA_INDEX"))

	algolia.Clear()

	os := []algoliasearch.Object{}

	for _, d := range ds.documents {
		body := d.Body

		for {
			stripped := reTag.ReplaceAll(body, []byte("$2"))
			if bytes.Equal(body, stripped) {
				break
			}
			body = stripped
		}

		if len(body) > 8000 {
			body = body[0:8000]
		}

		c := categories.Find(strings.Split(d.Slug, "/")[0])

		os = append(os, algoliasearch.Object{
			"objectID":       d.Slug,
			"category_slug":  c.Slug,
			"category_title": strings.Join(ds.Breadcrumbs(d.Slug), " » "),
			"body":           string(body),
			"slug":           string(d.Slug),
			"title":          string(d.Title),
			"url":            "/" + d.Slug,
		})
	}

	if _, err := algolia.AddObjects(os); err != nil {
		return err
	}

	return nil
}

func (ds *Documents) Breadcrumbs(slug string) []string {
	d, ok := ds.Find(slug)
	if !ok {
		return []string{}
	}

	bs := []string{}

	bs = append(bs, d.Category().Name)

	parts := strings.Split(d.Slug, "/")

	for i := 1; i < len(parts); i++ {
		if d, ok := ds.Find(strings.Join(parts[0:i], "/")); ok {
			bs = append(bs, d.Title)
		}
	}

	return bs
}

func (d *Document) Category() Category {
	return categories.Find(strings.Split(d.Slug, "/")[0])
}

func (d *Document) Level() int {
	return level(d.Slug)
}

func level(slug string) int {
	return len(strings.Split(slug, "/")) - 1
}

func parseDocument(path string, data []byte) (*Document, error) {
	name := strings.TrimSuffix(path, ".md")
	readme := strings.HasSuffix(path, "/README.md")
	slug := strings.TrimSuffix(strings.Replace(name, ".", "-", -1), "/README")

	d := &Document{
		Path: path,
		Slug: slug,
	}

	m := reDocument.FindSubmatch(data)

	if len(m) != 4 {
		return nil, nil
	}

	var front map[string]string

	if err := yaml.Unmarshal(m[1], &front); err != nil {
		return nil, err
	}

	d.Description = front["description"]
	d.Title = front["title"]

	d.Order = 50000

	if os, ok := front["order"]; ok {
		o, err := strconv.Atoi(os)
		if err != nil {
			return nil, err
		}
		d.Order = o
	}

	d.Tags = []string{}

	for _, t := range strings.Split(front["tags"], ",") {
		if tt := strings.TrimSpace(t); tt != "" {
			d.Tags = append(d.Tags, tt)
		}
	}

	sort.Strings(d.Tags)

	markdown := m[3]

	for _, n := range reMarkdownDiv.FindAllSubmatch(markdown, -1) {
		np := blackfriday.Run(n[2],
			blackfriday.WithExtensions(blackfriday.CommonExtensions|blackfriday.AutoHeadingIDs|blackfriday.LaxHTMLBlocks),
		)

		markdown = bytes.Replace(markdown, n[2], np, -1)
	}

	parsed := blackfriday.Run(markdown,
		blackfriday.WithExtensions(blackfriday.CommonExtensions|blackfriday.AutoHeadingIDs|blackfriday.LaxHTMLBlocks),
	)

	d.Body = reLink.ReplaceAllFunc(parsed, func(link []byte) []byte {
		if match := reLink.FindSubmatch(link); len(match) < 2 {
			return link
		} else {
			u, err := url.Parse(string(match[1]))
			if err != nil {
				return link
			}
			if u.Host == "" {
				u.Path = strings.TrimSuffix(u.Path, ".md")
				if readme {
					u.Path = fmt.Sprintf("/%s/%s", d.Slug, u.Path)
				}
			}
			return []byte(fmt.Sprintf("href=%q", u.String()))
		}
	})

	if d.Title == "" {
		if m := reTitle.FindStringSubmatch(string(d.Body)); len(m) > 1 {
			d.Title = m[1]
		}
	}

	if d.Title == "" {
		parts := strings.Split(d.Slug, "/")
		d.Title = strings.Title(strings.ReplaceAll(parts[len(parts)-1], "-", " "))
	}

	return d, nil
}
