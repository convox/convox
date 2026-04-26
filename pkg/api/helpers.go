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
	// MF-13 fix (R6 γ-4 A3 — RFC 6648 dual-read).
	//
	// X-Convox-TID is the multi-tenant boundary identifier set by Convox
	// Cloud (console3) on every proxied request and consumed by the rack
	// for namespace labeling, app-list scoping, service URL routing,
	// build-env injection, and pod-env injection. Hard rename would break
	// every Cloud-hosted customer.
	//
	// RFC 6648 (2012) deprecates the `X-` prefix for new HTTP headers.
	// Canonical going forward: `Convox-TID`. Both forms are accepted; the
	// canonical form wins when present. Console3 will migrate to
	// `Convox-TID` on its own cadence; the rack continues to read both
	// indefinitely until the migration is complete and a future release
	// can safely drop the legacy form.
	tid := c.Header("Convox-TID")
	if tid == "" {
		tid = c.Header("X-Convox-TID")
	}
	ctx := context.WithValue(c.Context(), structs.ConvoxTIDCtxKey, tid)
	// Propagate the JWT user claim set by the authenticate middleware so the
	// request-scoped Provider returned via WithContext can derive an audit
	// "actor" via ContextActor. Empty strings are skipped — ContextActor
	// falls back to "unknown" when the value is absent.
	if v, ok := c.Get(structs.ConvoxJwtUserParam).(string); ok && v != "" {
		ctx = context.WithValue(ctx, structs.ConvoxJwtUserCtxKey, v)
	}
	return ctx
}
