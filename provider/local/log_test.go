package local

import (
	"context"
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/convox/convox/pkg/options"
	"github.com/convox/convox/pkg/structs"
	"github.com/stretchr/testify/require"
)

func TestSubscribeLogsFilter(t *testing.T) {
	stream := fmt.Sprintf("test-%d", time.Now().UnixNano())
	ts := time.Now().UTC().Add(-time.Second)

	for _, m := range []string{"err-1", "ok-2", "err-3"} {
		logs.Append(stream, ts, "web-1", m)
	}

	tests := []struct {
		name   string
		filter string
		prefix bool
		want   string
	}{
		{"Match", "err", false, "err-1\nerr-3\n"},
		{"Case", "ERR", false, ""},
		{"Empty", "", false, "err-1\nok-2\nerr-3\n"},
		{"PrefixOnly", "web-1", true, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, w := io.Pipe()

			go subscribeLogs(context.Background(), w, stream, structs.LogsOptions{
				Filter: options.String(tt.filter),
				Follow: options.Bool(false),
				Prefix: options.Bool(tt.prefix),
				Since:  options.Duration(time.Minute),
			})

			data, err := io.ReadAll(r)
			require.NoError(t, err)
			require.Equal(t, tt.want, string(data))
		})
	}
}
