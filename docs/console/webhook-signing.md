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
`webhook_signing_key` rack parameter as the key. The signature is encoded as
hex and sent in the `Convox-Signature` header alongside a `Convox-Signature-Timestamp`
header carrying the UTC unix timestamp of the dispatch.

```
POST /webhooks/budget HTTP/1.1
Content-Type: application/json
Convox-Signature: 4b2c5f7a8b9d6e3a1f0c5e8d7b4a3c2f1e9d8c7b6a5f4e3d2c1b0a9e8d7c6b5a
Convox-Signature-Timestamp: 1714233600

{"event":"app:budget:cap","app":"myapp","actor":"alice@example.com",...}
```

To verify, recompute `HMAC-SHA256(webhook_signing_key, timestamp + "." + raw_body)`
and constant-time compare to the header value. The timestamp prefix prevents
replay across rotation events.

Example verification (Python):

```python
import hmac, hashlib, time

def verify(req, signing_key):
    sig = req.headers["Convox-Signature"]
    ts  = req.headers["Convox-Signature-Timestamp"]
    if abs(time.time() - int(ts)) > 300:
        return False  # too old, reject
    body = req.body  # raw bytes, before any JSON parse
    expected = hmac.new(
        signing_key.encode("utf-8"),
        f"{ts}.".encode("utf-8") + body,
        hashlib.sha256,
    ).hexdigest()
    return hmac.compare_digest(sig, expected)
```

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
introspection commands against rc4+ racks. See the 3.24.6 release notes.

## Cross-provider availability

In 3.24.6, signing is enabled on all 6 providers (AWS, Azure, GCP, DigitalOcean,
Metal, Local). Pre-3.24.6 racks on non-AWS providers did not sign webhooks even
when `webhook_signing_key` was set; receivers may have been configured to
accept unsigned payloads from those racks. Post-3.24.6, those receivers will
start receiving the `Convox-Signature` header. Either configure the receiver
with the rack's signing key (recommended) or explicitly accept unsigned
during the transition window.

## Receiver migration

Existing receivers that handle the `app:budget:cap` event family will see two
new event types in 3.24.6:

- `app:budget:auto-shutdown:dismissed` — sent when the recovery banner is
  dismissed via `convox budget dismiss-recovery` or the Console UI.
- `app:budget:auto-shutdown:breaker-cleared` — sent when a cap-raise clears
  the breaker mid-countdown (Decision 3, post-:fired recovery).

Receivers that fail-closed on unknown event types should either fail-open
(treat unknown events as informational) or be updated to handle the new types.

The `actor` field on `app:budget:auto-shutdown:*` events is now per-user (the
authenticated email of the operator who triggered the action) instead of the
historical `"rack-password"` constant. Receivers that key on actor for audit
should expect emails for Console3-driven mutations.

Webhooks are best-effort, fire-and-forget, and not retried. Receivers must
handle ordering by the `Convox-Signature-Timestamp` header (or by the `at`
field in the JSON body). The Events tab in the Console is best-effort
persistence for the same stream — Slack/Discord webhooks are the source of
truth on a gap.

## See Also

- [Webhooks](/configuration/webhooks) — webhook configuration reference
- [Rack Roles](/console/rack-roles) — Console RBAC and rack ownership
- [Budget Caps](/management/budget-caps) — events emitted to webhooks
