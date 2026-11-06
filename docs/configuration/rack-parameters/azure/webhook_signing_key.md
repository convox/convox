---
title: "webhook_signing_key"
description: "The webhook_signing_key Azure rack parameter sets a per-Rack HMAC secret that signs outbound webhook deliveries with a Convox-Signature header, unset by default."
slug: webhook_signing_key
url: /configuration/rack-parameters/azure/webhook_signing_key
---

# webhook_signing_key

## Description
The `webhook_signing_key` parameter sets a per-Rack HMAC secret that the Rack uses to sign every outbound webhook delivery. With this parameter set, the Rack adds a `Convox-Signature` HTTP header to each webhook POST. The header carries `t=<unix-ts>,v1=<hex>[,v1=<hex>]`, where `t` is the Unix timestamp at emit time and each `v1=<hex>` segment is the HMAC-SHA256 of `<timestamp>.<body>` computed with one configured signing key. Multiple `v1=` segments appear during key rotation; receivers verify against any one. See [Webhook Signing](/console/webhook-signing) for receiver-side verification examples.

When unset (the default), no signature header is emitted, so receivers cannot distinguish authentic Convox webhooks from spoofed payloads. Set this parameter and configure the same secret on your receiver to enable HMAC verification.

The value is treated as a credential. The Rack stores it as a Kubernetes Secret and injects it into the Rack API through that Secret, `convox rack params` masks it in terminal output (pass `--reveal` to show it), and telemetry emits only a salted hash of the value (an HMAC-SHA256 digest keyed per Rack), never the key itself.

## Default Value
The default value for `webhook_signing_key` is `""` (empty string). With an empty value the Rack sends webhooks without a `Convox-Signature` header, so receivers cannot HMAC-verify deliveries.

## Use Cases
- **Webhook authenticity verification**: Enable HMAC verification on receivers so a leaked webhook URL cannot be abused by a third party posting crafted payloads.
- **Compliance**: Signed deliveries help satisfy integrity and authenticity controls in frameworks such as PCI DSS and SOC 2; this parameter is the Rack-side enabler for that control.
- **Replay protection**: The `t=<unix-ts>` segment in `Convox-Signature` lets receivers reject deliveries outside a tolerance window to mitigate replay attacks.
- **Zero-downtime key rotation**: Configuring a comma-separated key list signs each delivery with every key, so receivers keep verifying while they migrate from the old key to the new one.

## Setting Parameters
Each key must be a lowercase hex string of at least 64 characters (32 bytes). Generate one with `openssl rand -hex 32` and set it:
```bash
$ convox rack params set webhook_signing_key=$(openssl rand -hex 32) -r rackName
Updating parameters... OK
```

The Rack validates the value before enabling signing. Keys that are too short, contain non-hex characters, match known placeholder values, or decode to low-entropy bytes are rejected. If an invalid value reaches the Rack, the API logs the validation error and delivers webhooks unsigned rather than failing.

To rotate to a new secret without disabling signing, set a comma-separated pair (`newkey,oldkey`, up to 4 keys) so receivers continue to verify against either segment during the rotation window:
```bash
$ convox rack params set webhook_signing_key="$NEW_KEY,$OLD_KEY" -r rackName
Updating parameters... OK
```
Then update receivers to the new key and run `convox rack params set webhook_signing_key="$NEW_KEY"` to retire the old segment.

Once set, the parameter cannot be cleared back to empty. The CLI rejects an empty value with `param 'webhook_signing_key' requires an explicit value (omit to keep current)`.

## Additional Information
- Available since Rack version 3.24.6.
- The value is stored as a Kubernetes Secret on the Rack, not a ConfigMap, and is excluded from plaintext parameter telemetry; only a per-Rack salted hash is emitted off-Rack.
- A key change rolls the Rack API Deployment as part of the parameter update, so deliveries after the update complete are signed with the updated key set.
- The signature is computed over the Unix timestamp, a literal `.`, then the raw request body. One `v1=` segment is emitted per configured key.
- Signing applies to every outbound webhook the Rack sends. Leaving the parameter unset preserves the previous behavior of unsigned deliveries; the feature is purely opt-in.

## See Also
- [Webhooks](/configuration/webhooks) for the webhook event catalog and delivery semantics
- [Webhook Signing](/console/webhook-signing) for header format details and receiver verification code
- [docker_hub_password](/configuration/rack-parameters/azure/docker_hub_password) for another credential parameter stored as a Kubernetes Secret
