---
title: "webhook_signing_key"
slug: webhook_signing_key
url: /configuration/rack-parameters/aws/webhook_signing_key
---

# webhook_signing_key

## Description
The `webhook_signing_key` parameter sets a per-rack HMAC secret that the api-pod uses to sign every outbound webhook delivery. With this parameter set, the rack adds a `Convox-Signature` HTTP header to each webhook POST. The header carries `t=<unix-ts>,v1=<hex1>[,v1=<hex2>]` — a Stripe-style multi-segment value where `t` is the Unix timestamp at emit time and each `v1=<hex>` segment is the HMAC-SHA256 of `<t>.<body>` keyed by the configured signing key. Multiple `v1=` segments may appear during key rotation; receivers verify against any one. See [Webhook Signing](/console/webhook-signing) for the receiver-side verification example.

When unset (the default), no signature header is emitted — receivers cannot distinguish authentic Convox webhooks from spoofed payloads. Set this parameter and configure the same secret on your receiver to enable HMAC verification.

The parameter value is treated as a credential and is stored only in a Kubernetes Secret on the rack — never in the plaintext ConfigMap and never in the rack's deploy-spec annotations. Rack telemetry (heartbeat to metrics.convox.com) emits a SHA-256 hash of the value to signal "key set" / "key rotated" without leaking the plaintext.

## Default Value
The default value for `webhook_signing_key` is `""` (empty string). Empty value means "no signature header on outbound webhooks" — receivers cannot HMAC-verify.

## Use Cases
- **Webhook authenticity verification**: Enable HMAC verification on receivers so a leaked webhook URL cannot be replayed by a third party with crafted payloads.
- **Compliance with PCI / SOC 2 webhook requirements**: Many compliance frameworks require signed webhook deliveries; this parameter is the rack-side enabler for that requirement.
- **Receiver-side replay protection**: The `t=<unix-ts>` segment embedded in `Convox-Signature` lets receivers reject deliveries outside a tolerance window (Convox recommends 5 minutes) to mitigate replay attacks.
- **Rotation testing**: Rotating this parameter during a maintenance window lets you exercise your receiver's HMAC-verification path with a known-bad signature (the OLD secret) and a known-good signature (the NEW secret) to validate end-to-end.

## Setting Parameters
The recommended secret is a 256-bit random value rendered as 64 hex characters. Generate one with:
```bash
$ openssl rand -hex 32
0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef
```

To set:
```bash
$ convox rack params set webhook_signing_key=0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef -r rackName
Updating parameters... OK
```

To clear (and disable signing):
```bash
$ convox rack params set webhook_signing_key='' -r rackName
Updating parameters... OK
```

## Additional Information
- The value is treated as sensitive: stored as a Kubernetes Secret (not a ConfigMap), never logged in plaintext, never serialized into rack deploy-spec annotations, and SHA-256-hashed before emission to telemetry.
- Rotation is safe at any time. The api-pod re-reads the Secret on each delivery, so a rotation takes effect on the next webhook event. Plan rotations to follow a "set new secret on receiver, rotate rack secret, observe verification success" sequence so no deliveries are lost.
- The signature scheme signs `fmt.Sprintf("%d.%s", t, body)` — the Unix timestamp followed by a literal `.` then the raw request body bytes. The header value `t=<unix-ts>,v1=<hex1>[,v1=<hex2>]` packs the timestamp and one-or-more hex-encoded HMAC-SHA256 outputs into a single header line; multiple `v1=` segments support zero-downtime key rotation.
- The signing applies to every outbound webhook event class: budget-cap events (`app:budget:set` / `:cap` / `:armed` / `:fired` / `:cancelled` / `:restored` / `:noop` / `:expired` / `:flap-suppressed` / `:failed` / `:simulated` / `:dismissed` / `:per-service-truncated`), release lifecycle (`release:promote`, `app:promote:completed` / `:errored` / `:cancelled`), scale-override (`app:scale-override:toggled` / `:honored`), and any future event class introduced post-3.24.6.
- If your receiver does not yet support HMAC verification, leaving this parameter unset preserves the pre-3.24.6 behavior (no signature header). The parameter is purely opt-in.

## Related Parameters
- [docker_hub_password](/configuration/rack-parameters/aws/docker_hub_password): Another rack-level credential that is stored in a Kubernetes Secret and SHA-256-hashed before telemetry emission.
- [cost_tracking_enable](/configuration/rack-parameters/aws/cost_tracking_enable): The cost-tracking accumulator emits webhook events; HMAC signing applies to those events when this parameter is set.

## Version Requirements
This parameter requires at least Convox rack version `3.24.6`.
