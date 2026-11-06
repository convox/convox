---
title: "webhook_signing_key"
description: "The webhook_signing_key Digital Ocean rack parameter sets an HMAC secret that signs outbound webhook deliveries with a Convox-Signature header, unset by default."
slug: webhook_signing_key
url: /configuration/rack-parameters/do/webhook_signing_key
---

# webhook_signing_key

## Description
The `webhook_signing_key` parameter sets a per-Rack HMAC secret that the Rack uses to sign every outbound webhook delivery. With this parameter set, the Rack adds a `Convox-Signature` HTTP header to each webhook POST. The header carries `t=<unix-ts>,v1=<hex1>[,v1=<hex2>]`, where `t` is the Unix timestamp at delivery time and each `v1=<hex>` segment is the HMAC-SHA256 of `<t>.<body>` keyed by one of the configured signing keys. Multiple `v1=` segments appear during key rotation; receivers verify against any one. See [Webhook Signing](/console/webhook-signing) for the receiver-side verification example.

When unset (the default), no signature header is emitted, so receivers cannot distinguish authentic Convox webhooks from spoofed payloads. Set this parameter and configure the same secret on your receiver to enable HMAC verification.

The value is treated as a credential. It is stored as a Kubernetes Secret on the Rack and masked in interactive `convox rack params` output. Piped output and the `--reveal` flag show the plaintext value.

## Default Value
The default value for `webhook_signing_key` is `""` (empty string). When empty, outbound webhooks carry no signature header.

## Use Cases
- **Webhook authenticity verification**: Enable HMAC verification on receivers so a leaked webhook URL cannot be used by a third party to deliver crafted payloads.
- **Compliance requirements**: Frameworks such as PCI and SOC 2 often require signed webhook deliveries; this parameter enables that on the Rack side.
- **Replay protection**: The `t=<unix-ts>` segment in `Convox-Signature` lets receivers reject deliveries outside a tolerance window.
- **Zero-downtime key rotation**: Comma-separated keys each produce their own `v1=` segment, so receivers keep verifying while you move from the old key to the new one.

## Setting Parameters
Each key must be a lowercase hex string of at least 64 characters (32 bytes). Generate one with:
```bash
$ openssl rand -hex 32
```

To set the `webhook_signing_key` parameter, use the following command:
```bash
$ convox rack params set webhook_signing_key=<64-char-hex-key> -r rackName
Updating parameters... OK
```
This command enables HMAC signing on all outbound webhook deliveries from the Rack.

To rotate to a new secret without disabling signing, set a comma-separated list (`newkey,oldkey`) so receivers continue to verify against either key during the rotation window:
```bash
$ convox rack params set webhook_signing_key="$NEW_KEY,$OLD_KEY" -r rackName
Updating parameters... OK
```
Once receivers hold the new key, set the parameter to the new key alone to retire the old one. Up to 4 comma-separated keys are supported.

## Additional Information
The Rack validates the key at API startup. Keys must be lowercase hex, even-length, at least 64 hex characters each, and at most 4 keys. Well-known placeholder values (such as all zeros or repeated patterns) and low-entropy values are rejected. If validation fails, the Rack logs the error and continues delivering webhooks unsigned rather than crashing.

The value is stored as a Kubernetes Secret in the Rack's namespace, not in a ConfigMap, and changing it rolls the Rack API Process so the new key takes effect immediately.

The signature is computed over the Unix timestamp, a literal `.`, then the raw request body. Signing applies to every outbound webhook the Rack sends. See [Webhooks](/configuration/webhooks) for the event catalog and [Webhook Signing](/console/webhook-signing) for receiver-side verification.

This parameter is available since Rack version `3.24.6`. Leaving it unset preserves the earlier behavior (no signature header), so the parameter is purely opt-in.

## See Also
- [Webhooks](/configuration/webhooks) for the webhook event catalog and delivery format
- [Webhook Signing](/console/webhook-signing) for verifying the `Convox-Signature` header on your receiver
- [docker_hub_password](/configuration/rack-parameters/do/docker_hub_password) for another credential-type Rack parameter stored as a Kubernetes Secret
