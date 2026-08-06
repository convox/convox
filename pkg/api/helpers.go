package api

import (
	"context"
	"fmt"
	"io"

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
