package api

import (
	"context"
	"fmt"
	"io"

	"github.com/convox/stdapi"
)

func renderStatusCode(w io.Writer, code int) error {
	_, err := fmt.Fprintf(w, "F1E49A85-0AD7-4AEF-A618-C249C6E6568D:%d\n", code)
	return err
}

func contextFrom(c *stdapi.Context) context.Context {
	return context.WithValue(c.Context(), "X-Convox-TID", c.Header("X-Convox-TID"))
}
