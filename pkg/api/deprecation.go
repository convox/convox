package api

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/convox/convox/pkg/structs"
	"github.com/convox/stdapi"
)

func deprecationSunsetDate() string {
	return structs.SunsetDate3250
}

// resolveAckByOverride derives the budget-handler actor from the JWT claim,
// with an optional ack_by form-param override. Non-empty ack_by emits
// RFC 8594 deprecation headers (the form-param path is deprecated in favor
// of per-user JWT in 3.25.0).
func resolveAckByOverride(c *stdapi.Context, app string) string {
	derived, _ := c.Get(structs.ConvoxJwtUserParam).(string)
	derived = strings.TrimSpace(derived)
	if derived == "" {
		derived = "unknown"
	}

	rawAckBy := strings.TrimSpace(formValue(c, "ack_by"))
	if rawAckBy == "" {
		return derived
	}

	c.Response().Header().Set("Deprecation", "true")
	c.Response().Header().Set("Sunset", deprecationSunsetDate())
	c.Response().Header().Set("Link", `<https://docs.convox.com/migration/ack-by-derivation>; rel="deprecation"; type="text/html"`)
	fmt.Printf("ns=api at=warn kind=ack_by_override app=%s client_supplied=%q jwt_user=%q\n",
		app, rawAckBy, derived)
	return rawAckBy
}

// formValue reads a form parameter, manually parsing DELETE request bodies
// since Go's stdlib skips body parsing for DELETE.
func formValue(c *stdapi.Context, name string) string {
	r := c.Request()
	if r.Method == http.MethodDelete && r.PostForm == nil &&
		strings.HasPrefix(r.Header.Get("Content-Type"), "application/x-www-form-urlencoded") {
		body, _ := io.ReadAll(r.Body)
		_ = r.Body.Close()
		r.Body = io.NopCloser(bytes.NewReader(body))
		if vals, err := url.ParseQuery(string(body)); err == nil {
			r.PostForm = vals
			if r.Form == nil {
				r.Form = url.Values{}
			}
			for k, v := range vals {
				r.Form[k] = append(r.Form[k], v...)
			}
		}
	}
	return c.Value(name)
}
