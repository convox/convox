package k8s

import (
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"time"

	"github.com/convox/convox/pkg/common"
	"github.com/convox/convox/pkg/structs"
	"github.com/pkg/errors"
)

func (p *Provider) Proxy(host string, port int, rw io.ReadWriter, opts structs.ProxyOptions) error {
	cn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", host, port), 3*time.Second)
	if err != nil {
		return errors.WithStack(err)
	}

	if common.DefaultBool(opts.TLS, false) {
		cn = tls.Client(cn, &tls.Config{})
	}

	if err := common.Pipe(cn, rw); err != nil {
		return errors.WithStack(err)
	}

	return nil
}
