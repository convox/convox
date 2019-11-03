package do

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"strings"
	"sync"
	"time"

	"github.com/convox/convox/pkg/common"
	"github.com/convox/convox/pkg/structs"
)

var sequenceTokens sync.Map

type elasticSearchResult struct {
	Hits struct {
		Hits []struct {
			Index  string `json:"_index"`
			Source struct {
				Log       string
				Stream    string
				Timestamp time.Time `json:"@timestamp"`
			} `json:"_source"`
		}
	}
}

func (p *Provider) Log(app, stream string, ts time.Time, message string) error {
	return nil
}

func (p *Provider) AppLogs(name string, opts structs.LogsOptions) (io.ReadCloser, error) {
	r, w := io.Pipe()

	go p.streamLogs(w, fmt.Sprintf("convox.%s.%s", p.Name, name), opts)

	return r, nil
}

func (p *Provider) streamLogs(w io.WriteCloser, index string, opts structs.LogsOptions) {
	defer w.Close()

	res, err := p.Elastic.Search(
		p.Elastic.Search.WithIndex(index),
		p.Elastic.Search.WithSize(5000),
	)
	if err != nil {
		fmt.Printf("err: %+v\n", err)
		return
	}
	defer res.Body.Close()

	data, err := ioutil.ReadAll(res.Body)
	if err != nil {
		fmt.Printf("err: %+v\n", err)
		return
	}

	fmt.Printf("string(data): %+v\n", string(data))

	var sres elasticSearchResult

	if err := json.Unmarshal(data, &sres); err != nil {
		fmt.Printf("err: %+v\n", err)
		return
	}

	for _, log := range sres.Hits.Hits {
		prefix := ""

		if common.DefaultBool(opts.Prefix, false) {
			prefix = fmt.Sprintf("%s %s ", log.Source.Timestamp.Format(time.RFC3339), strings.ReplaceAll(log.Source.Stream, ".", "/"))
		}

		fmt.Fprintf(w, "%s%s", prefix, log.Source.Log)
	}
}
