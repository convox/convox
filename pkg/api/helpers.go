package api

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/convox/convox/pkg/options"
	"github.com/convox/convox/pkg/structs"
	"github.com/convox/stdapi"
)

func renderStatusCode(w io.Writer, code int) error {
	_, err := fmt.Fprintf(w, "F1E49A85-0AD7-4AEF-A618-C249C6E6568D:%d\n", code)
	return err
}

// requestTID reads the tenant ID stamped on the request. X-Convox-TID is the
// form every producer sets; the unprefixed spelling has none, so on a rack
// serving tenants it can only be caller input and is ignored there.
func requestTID(c *stdapi.Context) string {
	if tid := c.Header("X-Convox-TID"); tid != "" {
		return tid
	}
	if options.GetFeatureGates()[options.FeatureGateTid] {
		return ""
	}
	return c.Header("Convox-TID")
}

func contextFrom(c *stdapi.Context) context.Context {
	ctx := context.WithValue(c.Context(), structs.ConvoxTIDCtxKey, requestTID(c))
	if v, ok := c.Get(structs.ConvoxJwtUserParam).(string); ok && v != "" {
		ctx = context.WithValue(ctx, structs.ConvoxJwtUserCtxKey, v)
	}
	return ctx
}

// copyLogs stops the stream when the client hangs up. After the stream ends it
// keeps reading until then: closing with unread input resets the connection.
func copyLogs(c *stdapi.Context, r io.ReadCloser) error {
	ws := c.Websocket()
	gone := make(chan struct{})

	go func() {
		defer ws.Close()

		ws.SetPingHandler(func(string) error { return nil })

		for {
			if _, _, err := ws.NextReader(); err != nil {
				close(gone)
				r.Close()
				return
			}
		}
	}()

	_, err := io.Copy(c, r)

	select {
	case <-gone:
		return nil
	default:
	}

	_ = ws.NetConn().SetReadDeadline(time.Now().Add(5 * time.Minute))

	return err
}
