---
title: "syslog_tls_verify"
description: "The syslog_tls_verify AWS rack parameter controls certificate verification for a tcp+tls syslog endpoint, defaulting to true."
slug: syslog_tls_verify
url: /configuration/rack-parameters/aws/syslog_tls_verify
---

# syslog_tls_verify

## Description

The `syslog_tls_verify` parameter controls whether Fluentd verifies the certificate of a `tcp+tls` endpoint set in [`syslog`](/configuration/rack-parameters/aws/syslog). When it is `true`, Fluentd checks the endpoint's certificate chain against the public certificate authorities trusted by the log forwarder image and does not send logs to an endpoint that fails the check. Set it to `false` to forward to an endpoint with a self-signed or private-CA certificate.

## Default Value

The default value is `true`.

An endpoint whose certificate chains to a public certificate authority, and that sends its intermediate certificates, receives logs with no change. An endpoint with a self-signed or private-CA certificate, or one that sends only its leaf certificate, receives nothing until `syslog_tls_verify` is set to `false`.

## Use Cases

- **Public syslog services**: leave the default so logs go only to an endpoint whose certificate is valid for its hostname.
- **Self-signed or private-CA endpoints**: set `false` to forward to a receiver inside your network that uses a certificate a public CA did not issue.

## Setting Parameters

```bash
$ convox rack params set syslog_tls_verify=false -r rackName
Updating parameters... OK
```

The change rolls the Fluentd pods on every node. CloudWatch delivery continues throughout.

## Viewing Current Configuration

```bash
$ convox rack params -r rackName
```

## Additional Information

- **Scope:** applies only to `tcp+tls` endpoints. It has no effect on `tcp` or `udp` endpoints, or when `syslog` is not set.
- **Hostname check:** with either value, the endpoint's certificate must name the host in the `syslog` URL. Fluentd sends the hostname as SNI unless the URL uses an IP address, and connects with TLS 1.2 or later.
- **Intermediate certificates:** verification uses only the certificate authorities in the image. An endpoint must send its intermediate certificates; one that sends only its leaf certificate fails even when its CA is public.
- **Failing endpoint:** an endpoint that fails verification receives nothing. Fluentd logs one warning per minute naming the TLS error and keeps writing to CloudWatch, so `convox logs` and `convox rack logs` are unaffected.
- **Validation:** boolean. The CLI rejects other values with `param 'syslog_tls_verify' must be 'true' or 'false' (got "<value>")`.
- **Version:** requires Rack version `3.25.10` or later and CLI version `3.25.10` or later. An older CLI rejects the parameter with `unknown parameter 'syslog_tls_verify' for aws provider`.
- **Downgrade:** downgrading below `3.25.10` removes the parameter with `NOTICE: removing parameters not supported by version <version>: syslog_tls_verify` on stderr. A Rack managed through the Console keeps the stored value and applies it again on the next update to `3.25.10` or later. A self-managed Rack drops the value, so set it again after upgrading.
- **Providers:** AWS only. GCP, Azure and DigitalOcean Racks do not have this parameter.

## See Also

- [syslog](/configuration/rack-parameters/aws/syslog) for the endpoint and what Fluentd forwards
- [Logging](/configuration/logging#log-forwarding) for log forwarding on every provider
- [fluentd_disable](/configuration/rack-parameters/aws/fluentd_disable) for turning Fluentd off
