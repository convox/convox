---
title: "syslog"
description: "The syslog Digital Ocean rack parameter sets the endpoint to forward rack logs to a syslog server, such as tcp+tls://example.org:1234, disabled by default."
slug: syslog
url: /configuration/rack-parameters/do/syslog
---

# syslog

## Description
The `syslog` parameter specifies the endpoint to forward logs to a syslog server (e.g. **tcp+tls://example.org:1234**).

## Default Value
The default value for `syslog` is `""`.

## Accepted Values
A URL made of `tcp://`, `tcp+tls://` or `udp://` (lowercase), a host name or IPv4 address, and an optional `:<port>`, such as `tcp+tls://logs.example.com:6514`. The port defaults to `514` when the URL leaves it out. From the CLI, several endpoints can be given separated by commas, and each receives every line; the Console form takes one URL.

The CLI rejects any other value:

```text
ERROR: param 'syslog' must be tcp://, tcp+tls:// or udp:// followed by a host and optional port, such as tcp+tls://logs.example.com:6514 (got "udp+tls://logs.example.com:6514")
```

`--force` does not bypass the check, which requires CLI 3.25.12 or later. A value set with an older CLI or through `convox rack install` is not checked, and one with another scheme, such as `udp+tls`, forwards nothing.

## Use Cases
- **Centralized Logging**: Forward logs to a central syslog server for easier monitoring and analysis.
- **Compliance**: Meet compliance requirements by ensuring all logs are collected and stored in a central location.

## Setting Parameters
To set the `syslog` parameter, use the following command:
```bash
$ convox rack params set syslog=tcp+tls://example.org:1234 -r rackName
Updating parameters... OK
```
The change rolls the Fluentd pods on every node.

## Additional Information
When set to `""`, syslog forwarding is not enabled. Ensure that the syslog endpoint is reachable and configured to receive logs from your Convox Rack.

- **What is forwarded:** Fluentd forwards the container output of every App's Services, Timers, `convox run` Processes and Builds, and of the Rack's own system Pods such as the API, router and resolver. It does not collect containerized Resources such as `postgres` or `redis`, Kubernetes add-ons such as CoreDNS, or a Service whose name starts with `fluentd` or `cloudwatch-agent`. The Rack's own event lines, such as deploy state changes, are written to Elasticsearch directly and are not forwarded. Every router line, access lines included, is forwarded.
- **Message format:** each line is sent with facility `user` and severity `info`. The syslog hostname is `<rack>.<app>` and the tag is `<type>/<name>/<pod>`, cut to 32 characters: `service/web/<pod>` for a Service, `timer/<timer>/<pod>` for a Timer, and `process/<service>/<pod>` for a `convox run` Process. Over `tcp` and `tcp+tls`, a line of up to 16,384 bytes arrives whole. Over `udp`, each message, header and tag included, is cut at 1,024 bytes.
- **TLS:** from Rack version `3.25.11`, Fluentd verifies a `tcp+tls` endpoint's certificate against public certificate authorities. For an endpoint with a self-signed or private-CA certificate, set [`syslog_tls_verify`](/configuration/rack-parameters/do/syslog_tls_verify) to `false`.
- **Masking:** [`router_log_redact_params`](/configuration/rack-parameters/do/router_log_redact_params) masks query parameter values in router lines before they are stored or forwarded.
- **Failing endpoint:** when a `tcp` or `tcp+tls` endpoint refuses the connection, is unreachable, fails certificate verification or stops reading, Fluentd drops the line it was sending, logs a warning, skips syslog output for 60 seconds, then reconnects. Lines written during the pause are not resent. Each failure delays Elasticsearch delivery on that node by up to about 10 seconds, and nothing is lost from Elasticsearch. A `udp` endpoint that is down returns no error, so lines sent to it are lost without a warning.

## See Also
- [syslog_tls_verify](/configuration/rack-parameters/do/syslog_tls_verify) for certificate verification on `tcp+tls` endpoints
- [router_log_redact_params](/configuration/rack-parameters/do/router_log_redact_params) for masking query parameter values in router logs
- [Logging](/configuration/logging#log-forwarding) for log forwarding on every provider
- [fluentd_memory](/configuration/rack-parameters/do/fluentd_memory) for Fluentd memory on high log volume
