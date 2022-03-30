package api_test

import (
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"testing"

	"github.com/convox/convox/pkg/structs"
	"github.com/convox/stdsdk"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestProxy(t *testing.T) {
	tests := map[string]struct {
		size int64
	}{
		"0b":    {size: 0},
		"1b":    {size: 1},
		"128b":  {size: 128},
		"500b":  {size: 500},
		"1kb":   {size: 1000},
		"1kib":  {size: 1024},
		"2kib":  {size: 1024 * 2},
		"8kib":  {size: 1024 * 8},
		"16kib": {size: 1024 * 16},
		"1mib":  {size: 1024 * 1024},
		"8mib":  {size: 1024 * 1024 * 8},
	}

	for desc, tt := range tests {
		t.Run(desc, func(t *testing.T) {
			testServer(t, func(c *stdsdk.Client, p *structs.MockProvider) {
				ro := stdsdk.RequestOptions{
					Body: io.LimitReader(rand.New(rand.NewSource(1028890720402726901)), tt.size),
				}

				p.On("Proxy", "host", 5000, mock.Anything, structs.ProxyOptions{}).Return(nil).Run(func(args mock.Arguments) {
					rw := args.Get(2).(io.ReadWriter)
					// "echo"
					n, err := io.Copy(rw, rw)
					require.NoError(t, err)
					require.Equal(t, tt.size, int64(n))
				})
				r, err := c.Websocket("/proxy/host/5000", ro)
				require.NoError(t, err)
				data, err := ioutil.ReadAll(r)
				require.NoError(t, err)
				require.Len(t, data, int(tt.size))
			})
		})
	}
}

func TestProxyError(t *testing.T) {
	testServer(t, func(c *stdsdk.Client, p *structs.MockProvider) {
		p.On("Proxy", "host", 5000, mock.Anything, structs.ProxyOptions{}).Return(fmt.Errorf("err1"))
		r, err := c.Websocket("/proxy/host/5000", stdsdk.RequestOptions{})
		require.NoError(t, err)
		d, err := ioutil.ReadAll(r)
		require.NoError(t, err)
		require.Equal(t, []byte("ERROR: err1\n"), d)
	})
}
