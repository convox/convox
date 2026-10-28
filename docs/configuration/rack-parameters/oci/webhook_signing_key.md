---
title: "webhook_signing_key"
description: "The webhook_signing_key Oracle Cloud rack parameter sets a per-Rack HMAC secret that signs outbound webhook deliveries with a Convox-Signature header, unset by default."
slug: webhook_signing_key
url: /configuration/rack-parameters/oci/webhook_signing_key
---

# webhook_signing_key

## Description
The `webhook_signing_key` parameter sets a per-Rack HMAC secret that the Rack uses to sign every outbound webhook delivery. With this parameter set, the Rack adds a `Convox-Signature` HTTP header to each webhook POST. The header carries `t=<unix-ts>,v1=<hex1>[,v1=<hex2>]`, where `t` is the Unix timestamp at emit time and each `v1=<hex>` segment is the HMAC-SHA256 of `<t>.<body>` keyed by a configured signing key. Multiple `v1=` segments appear during key rotation; receivers verify against any one. See [Webhook Signing](/console/webhook-signing) for the receiver-side verification example.

When unset (the default), no signature header is emitted, so receivers cannot distinguish authentic Convox webhooks from spoofed payloads. Set this parameter and configure the same secret on your receiver to enable HMAC verification.

The parameter value is treated as a credential. The Rack stores it as a Kubernetes Secret, masks it in interactive `convox rack params` output (pass `--reveal` to display the value; piped output is not masked), and emits only a hash of the value to telemetry, never the key itself.

## Default Value
The default value for `webhook_signing_key` is `""` (empty string). With an empty value the Rack sends no signature header on outbound webhooks, so receivers cannot HMAC-verify.

## Use Cases
- **Webhook authenticity verification**: Enable HMAC verification on receivers so a leaked webhook URL cannot be exploited by a third party with crafted payloads.
- **Compliance with PCI / SOC 2 webhook requirements**: Many compliance frameworks require signed webhook deliveries; this parameter is the Rack-side enabler for that requirement.
- **Receiver-side replay protection**: The `t=<unix-ts>` segment embedded in `Convox-Signature` lets receivers reject deliveries outside a tolerance window to mitigate replay attacks.

## Setting Parameters
The key must be lowercase hex, at least 64 hex characters (32 bytes). Generate one with:
```bash
$ openssl rand -hex 32
a3f8c2e19b7d4e605c1f8a2d7e9b3c640d5a8f172c6e9b48d1f7a3e584b0c9f2
```

To set:
```bash
$ convox rack params set webhook_signing_key=a3f8c2e19b7d4e605c1f8a2d7e9b3c640d5a8f172c6e9b48d1f7a3e584b0c9f2 -r rackName
Updating parameters... OK
```

To rotate to a new secret without disabling signing, set a comma-separated pair (`newkey,oldkey`) so receivers continue to verify against either segment during the rotation window:
```bash
$ convox rack params set webhook_signing_key="$NEW_KEY,$OLD_KEY" -r rackName
Updating parameters... OK
```
Then update receivers to the new key and run `convox rack params set webhook_signing_key="$NEW_KEY"` to retire the old segment. Up to 4 comma-separated keys are supported during rotation.

## Additional Information
- Available since Rack version 3.24.6.
- The Rack validates the key at startup. Placeholder values (such as all zeros or repeated patterns), keys shorter than 64 hex characters, uppercase or non-hex characters, and low-entropy values are rejected. A rejected key does not break webhook delivery; the Rack logs a warning and continues delivering webhooks unsigned, so verify your receiver observes the `Convox-Signature` header after setting the parameter.
- The value is stored as a Kubernetes Secret on the Rack, never in a ConfigMap, and is masked in interactive `convox rack params` output unless `--reveal` is passed. Telemetry receives a hash of the value, never the key itself.
- Changing the key triggers a rolling restart of the Rack API, so a rotation takes effect within minutes of the parameter update completing.
- The signature is computed over the Unix timestamp, a literal `.`, then the raw request body. Each configured key produces its own `v1=` segment in the header, which supports zero-downtime key rotation.
- Signing applies to every outbound webhook the Rack sends. See [Webhooks](/configuration/webhooks) for the event types.
- If your receiver does not yet support HMAC verification, leaving this parameter unset preserves the default behavior (no signature header). The parameter is purely opt-in.

## See Also
- [Webhook Signing](/console/webhook-signing): Receiver-side verification, rotation guidance, and code samples.
- [Webhooks](/configuration/webhooks): The app-level events Racks deliver as webhooks.
- [docker_hub_password](/configuration/rack-parameters/oci/docker_hub_password): Another Rack-level credential stored as a Kubernetes Secret and masked in parameter output.
