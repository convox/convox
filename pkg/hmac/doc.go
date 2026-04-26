// Package hmac provides Stripe-style HMAC-SHA256 signing for outbound
// Convox webhook payloads.
//
// Webhook signing follows the industry-standard "sign exact bytes received"
// pattern. The rack signs HMAC-SHA256(key, fmt.Sprintf("%d.%s", t, body))
// where t is a Unix timestamp; the Convox-Signature header carries
// t=<ts>,v1=<hex>[,v1=<hex2>]. Receivers verify against the raw request
// body bytes plus the timestamp from the header.
//
// Stdlib collision: the Go standard library defines crypto/hmac. Callers
// importing this package alongside crypto/hmac MUST alias one of them.
// Repository convention is to alias this package as cxhmac:
//
//	import (
//	    "crypto/hmac"
//	    cxhmac "github.com/convox/convox/pkg/hmac"
//	)
//
// See https://docs.convox.com/configuration/webhooks#signing.
package hmac
