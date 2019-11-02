package do

import (
	"io"
	"io/ioutil"
	"strings"
	"sync"
	"time"

	"github.com/convox/convox/pkg/structs"
)

var sequenceTokens sync.Map

func (p *Provider) Log(app, stream string, ts time.Time, message string) error {
	return nil
}

func (p *Provider) AppLogs(name string, opts structs.LogsOptions) (io.ReadCloser, error) {
	return ioutil.NopCloser(strings.NewReader("")), nil
}
