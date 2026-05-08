---
title: "Webhook Signing"
slug: webhook-signing
url: /console/webhook-signing
---
# Webhook Signing

Outbound webhooks from Convox racks (budget events, deploy notifications,
auto-shutdown lifecycle) are signed when a `webhook_signing_key` is configured
on the rack. Receivers verify the `Convox-Signature` header to confirm the
payload originated from your rack and was not tampered with in transit.

## Signing <a id="signing"></a>

The rack signs each webhook payload with HMAC-SHA256 using the
`webhook_signing_key` rack parameter as the key. Both the timestamp and
the signature(s) are packed into a single `Convox-Signature` header as
comma-separated `key=value` segments:

```
Convox-Signature: t=<unix-ts>,v1=<hex1>[,v1=<hex2>]
```

- `t=<unix-ts>` — UTC unix timestamp of dispatch (seconds since epoch).
- `v1=<hex>` — hex-encoded HMAC-SHA256 signature. The signed input is
  `fmt.Sprintf("%d.%s", t, body)` — the timestamp, a literal `.`, then
  the raw response body bytes. Multiple `v1=` segments may appear when
  the rack is in the middle of a key rotation (one signature per active
  key; up to 4 keys are supported per rotation, see "Rotation depth"
  below). Receivers verify against ANY one of the listed `v1=` values.

Example header (HTTP-handler event — `app:budget:reset` carries the
JWT-derived `actor` from the operator who ran `convox budget reset`):

```
POST /webhooks/budget HTTP/1.1
Content-Type: application/json
Convox-Signature: t=1714233600,v1=4b2c5f7a8b9d6e3a1f0c5e8d7b4a3c2f1e9d8c7b6a5f4e3d2c1b0a9e8d7c6b5a

{"action":"app:budget:reset","status":"success","timestamp":"2026-04-27T10:30:00Z","data":{"app":"myapp","actor":"alice@example.com",...}}
```

To verify, parse the header, recompute
`HMAC-SHA256(webhook_signing_key, fmt.Sprintf("%d.%s", t, body))`,
hex-encode, and constant-time compare against any `v1=` segment.
Reject if the timestamp is outside your tolerance window (Convox
recommends 5 minutes).

Example verification (Python):

```python
import hmac, hashlib, time

def parse_header(header):
    """Return (t, [sigs]) from 't=<n>,v1=<hex>[,v1=<hex>]'."""
    t = None
    sigs = []
    for part in header.split(","):
        k, _, v = part.strip().partition("=")
        if k == "t":
            t = int(v)
        elif k == "v1":
            sigs.append(v)
    return t, sigs

def verify(req, signing_key):
    header = req.headers["Convox-Signature"]
    t, sigs = parse_header(header)
    if t is None or not sigs:
        return False
    if abs(time.time() - t) > 300:
        return False  # too old, reject
    body = req.body  # raw bytes, before any JSON parse
    expected = hmac.new(
        signing_key.encode("utf-8"),
        f"{t}.".encode("utf-8") + body,
        hashlib.sha256,
    ).hexdigest()
    return any(hmac.compare_digest(s, expected) for s in sigs)
```

Example verification (Go):

```go
package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strconv"
	"strings"
	"time"
)

const maxAgeSeconds = 300

func parseHeader(header string) (int64, []string) {
	var t int64
	var sigs []string
	for _, part := range strings.Split(header, ",") {
		k, v, ok := strings.Cut(strings.TrimSpace(part), "=")
		if !ok {
			continue
		}
		switch k {
		case "t":
			t, _ = strconv.ParseInt(v, 10, 64)
		case "v1":
			sigs = append(sigs, v)
		}
	}
	return t, sigs
}

// Verify returns nil when the signature is valid and the timestamp is within
// the 5-minute tolerance window. Pass signingKey as a UTF-8 string and body
// as the raw (unparsed) request body bytes.
func Verify(header, signingKey string, body []byte) error {
	t, sigs := parseHeader(header)
	if t == 0 || len(sigs) == 0 {
		return errors.New("missing timestamp or signature")
	}
	if abs(time.Now().Unix()-t) > maxAgeSeconds {
		return errors.New("timestamp outside 5-minute tolerance window")
	}
	mac := hmac.New(sha256.New, []byte(signingKey))
	mac.Write([]byte(strconv.FormatInt(t, 10) + "."))
	mac.Write(body)
	expected := hex.EncodeToString(mac.Sum(nil))
	for _, sig := range sigs {
		if hmac.Equal([]byte(sig), []byte(expected)) {
			return nil
		}
	}
	return errors.New("signature mismatch")
}

func abs(n int64) int64 {
	if n < 0 {
		return -n
	}
	return n
}
```

Example verification (Node.js / TypeScript):

```typescript
import { createHmac, timingSafeEqual } from "crypto";

const MAX_AGE_SECONDS = 300;

function parseHeader(header: string): { t: number; sigs: string[] } {
  let t = 0;
  const sigs: string[] = [];
  for (const part of header.split(",")) {
    const [k, v] = part.trim().split("=");
    if (k === "t") t = parseInt(v, 10);
    else if (k === "v1") sigs.push(v);
  }
  return { t, sigs };
}

/**
 * Returns true when the signature is valid and the timestamp is within the
 * 5-minute tolerance window.
 *
 * @param header  The raw Convox-Signature header value.
 * @param key     The webhook_signing_key rack parameter value (UTF-8 string).
 * @param body    The raw request body as a Buffer or string.
 */
export function verify(header: string, key: string, body: Buffer | string): boolean {
  const { t, sigs } = parseHeader(header);
  if (!t || sigs.length === 0) return false;
  if (Math.abs(Date.now() / 1000 - t) > MAX_AGE_SECONDS) return false; // 5-minute tolerance

  const mac = createHmac("sha256", key);
  mac.update(`${t}.`);
  mac.update(body);
  const expected = mac.digest("hex");
  const expectedBuf = Buffer.from(expected, "utf8");

  return sigs.some((sig) => {
    try {
      return timingSafeEqual(Buffer.from(sig, "utf8"), expectedBuf);
    } catch {
      return false; // length mismatch — safe to reject
    }
  });
}
```

Example verification (shell / openssl):

```bash
#!/usr/bin/env bash
# Verify a Convox webhook payload from shell.
#
# Usage: CONVOX_KEY=<signing_key> verify_convox_webhook "<header>" "<body>"
#
# Requires: openssl, xxd, awk

MAX_AGE=300  # 5-minute tolerance

verify_convox_webhook() {
  local header="$1" body="$2"
  local t="" sig=""

  # Parse t= and first v1= from the header
  for seg in $(echo "$header" | tr ',' '
'); do
    key="${seg%%=*}"; val="${seg#*=}"
    [[ "$key" == "t"  ]] && t="$val"
    [[ "$key" == "v1" && -z "$sig" ]] && sig="$val"
  done

  [[ -z "$t" || -z "$sig" ]] && { echo "INVALID: missing fields"; return 1; }

  # 5-minute timestamp tolerance
  now=$(date +%s)
  age=$(( now - t < 0 ? t - now : now - t ))
  (( age > MAX_AGE )) && { echo "INVALID: timestamp too old ($age s)"; return 1; }

  # Compute HMAC-SHA256: key=$CONVOX_KEY, input="${t}.${body}"
  signed_input="${t}.${body}"
  expected=$(printf '%s' "$signed_input"     | openssl dgst -sha256 -hmac "$CONVOX_KEY" -binary     | xxd -p -c 256)

  if [[ "$expected" == "$sig" ]]; then
    echo "VALID"
  else
    echo "INVALID: signature mismatch"
    return 1
  fi
}
```

> **Note:** The shell example uses a single-pass `printf | openssl` pipeline.
> Some openssl builds behave differently with `-hmac` vs `-mac hmac -macopt key:...`;
> if your environment requires the latter form, replace the `openssl dgst` line with:
> `openssl dgst -sha256 -mac hmac -macopt "key:$CONVOX_KEY" -binary`.


The multi-`v1=` form is what enables zero-downtime key rotation: when
rotating, configure the new key on the rack and BOTH keys (old + new)
on the receiver. The rack will sign with both for a grace window;
receiver accepts either; once the receiver has fully cut over, the old
key is removed from rack config and signing collapses back to one
`v1=`.

## Rotation depth

A rack accepts up to 4 active webhook signing keys at once (bumped from 2
in 3.24.6). The rotation pattern is unchanged — operators stack a
new key, update receivers in lockstep, then drop the oldest — but a
4-deep rotation depth handles the common case of staging long rollouts
across multiple receiver fleets without losing the original key during
the transition.

Setting more than 4 keys (e.g. `webhook_signing_key=k1,k2,k3,k4,k5`) is
rejected with a structured `audit_type=webhook_signing_key:eviction
reason=key_count_exceeded max=4 evicted_count=N` audit row on api-pod
stdout. The audit row contains the count only — no key bytes, hashes, or
prefixes — so operators can grep for the misconfiguration without leaking
secret material to logs (per F-SEC-4).

Wire size: 4 active keys produce a `Convox-Signature` header of about 280
bytes (4 × 64 hex chars + delimiters + timestamp), well under the 8KB
header limit on Cloudflare and the 4KB baseline on AWS Lambda receivers.
Per-event CPU cost is 4 HMAC-SHA256 operations vs 2 — negligible relative
to the network round-trip.

### Downgrade caveat

3.24.5 and earlier did not have webhook signing at all. A rack on 3.24.6
with any `webhook_signing_key` set blocks any downgrade below 3.24.6 —
the parameter is unknown to older versions. To downgrade, clear the
parameter first (`convox rack params set webhook_signing_key=`) and
reconfigure receivers to accept unsigned payloads during the transition.
This caveat is unchanged from the 3.24.6 GA — bumping the rotation depth
from 2 to 4 in the polish wave does not introduce a new downgrade trap;
the same "no signing pre-3.24.6" boundary applies.

If the operator rotates the webhook_signing_key to >2 keys and then
downgrades to a hypothetical future N+1 release that rolls the cap back
to 2 (no such release exists today), the trim must be performed before
downgrade.

## Configuring the signing key

Set the rack parameter:

```bash
$ convox rack params set webhook_signing_key=$(openssl rand -hex 32)
```

The rack uses the value as-is — any string of sufficient entropy works. Convox
recommends a 32-byte random hex string. Rotate by running the same command with
a new value; receivers must update their copy of the key in lockstep, since
old payloads cannot be re-signed.

The CLI masks the value in `convox rack params` output as of 3.24.6. Older CLIs
print the value plaintext to the TTY — upgrade the CLI before running param
introspection commands against 3.24.6 racks. See the 3.24.6 release notes.

## Cross-provider availability

In 3.24.6, signing is enabled on all 6 providers (AWS, Azure, GCP, DigitalOcean,
Metal, Local). Pre-3.24.6 racks on non-AWS providers did not sign webhooks even
when `webhook_signing_key` was set; receivers may have been configured to
accept unsigned payloads from those racks. Post-3.24.6, those receivers will
start receiving the `Convox-Signature` header. Either configure the receiver
with the rack's signing key (recommended) or explicitly accept unsigned
during the transition window.

## Receiver migration

Existing receivers will see seven new event types in 3.24.6. See
[Webhooks event catalog](/configuration/webhooks#event-catalog) for the
canonical payload shapes; the migration-relevant additions are:

- `app:budget:auto-shutdown:dismissed` — sent when the recovery banner is
  dismissed via `convox budget dismiss-recovery` or the Console UI.
- `app:budget:breaker-cleared` — top-level event (NOT a sub-type of
  `auto-shutdown`); sent when a cap-raise clears the deploy circuit
  breaker (both during the armed countdown and post-`:fired`).
- `app:budget:per-service-truncated` — emitted when the accumulator's
  per-service breakdown table exceeds its bounded-cardinality cap and
  drops entries from this month's persisted breakdown.
- `app:promote:completed`, `app:promote:errored`, `app:promote:cancelled`
  — terminal-state events from the rack-side rollout watcher; the
  pre-existing `release:promote` (status=start) event is unchanged.
- `app:scale-override:toggled`, `app:scale-override:honored` — emitted on
  the per-service scale-override toggle (handler) and at deploy time
  when an active override is honored (render-path; `actor: "system"`).

Receivers that fail-closed on unknown event types should either fail-open
(treat unknown events as informational) or be updated to handle the new types.

The `actor` field on `app:budget:*` events is now per-user (the
authenticated email of the operator who triggered the action) instead of the
historical `"rack-password"` constant. Receivers that key on actor for audit
should expect emails for Console-driven mutations.

Webhooks are best-effort, fire-and-forget, and not retried. Receivers must
handle ordering by the `timestamp` field in the JSON body (or by parsing
the `t=<unix-ts>` segment from the `Convox-Signature` header — both
reflect the same rack-side emit time). The Events tab in the Console is
best-effort persistence for the same stream — Slack/Discord webhooks are
the source of truth on a gap.

## See Also

- [Webhooks](/configuration/webhooks) — webhook configuration reference
- [Rack Roles](/console/rack-roles) — Console RBAC and rack ownership
- [Budget Caps](/management/budget-caps) — events emitted to webhooks
