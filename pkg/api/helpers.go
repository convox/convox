package api

import (
	"context"
	"fmt"
	"io"

	"github.com/convox/convox/pkg/structs"
	"github.com/convox/stdapi"
)

func renderStatusCode(w io.Writer, code int) error {
	_, err := fmt.Fprintf(w, "F1E49A85-0AD7-4AEF-A618-C249C6E6568D:%d\n", code)
	return err
}

func contextFrom(c *stdapi.Context) context.Context {
	// Read both header names for backward compat (RFC 6648 migration).
	tid := c.Header("Convox-TID")
	if tid == "" {
		tid = c.Header("X-Convox-TID")
	}
	ctx := context.WithValue(c.Context(), structs.ConvoxTIDCtxKey, tid)
	if v, ok := c.Get(structs.ConvoxJwtUserParam).(string); ok && v != "" {
		ctx = context.WithValue(ctx, structs.ConvoxJwtUserCtxKey, v)
	}
	return ctx
}
