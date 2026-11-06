---
title: "syslog_access_logs"
description: "The syslog_access_logs AWS rack parameter also forwards the public router's access logs to the syslog endpoint, defaulting to false."
slug: syslog_access_logs
url: /configuration/rack-parameters/aws/syslog_access_logs
---

# syslog_access_logs

## Description

The `syslog_access_logs` parameter controls whether Fluentd forwards the access log lines of the public `ingress-nginx` router to the endpoint set in [`syslog`](/configuration/rack-parameters/aws/syslog). When it is `true`, the endpoint receives one JSON line per request. With either value, the lines go to the `/nginx-access-logs` stream of the Rack system log group `/convox/<rack>/system`, which `convox rack logs` reads.

The internal router's access lines, and the public router's error and controller lines, reach `syslog` with this parameter unset.

## Default Value

The default value is `false`. `convox rack params` does not list the parameter until it is set.

## Use Cases

- **Request logs in your log aggregator**: keep the router's per-request records in the same system that receives your App logs over `syslog`.

## Setting Parameters

```bash
$ convox rack params set syslog_access_logs=true -r rackName
Updating parameters... OK
```

With `syslog` set, the change rolls the Fluentd pods on every node. With `syslog` unset, the parameter has no effect.

## Viewing Current Configuration

```bash
$ convox rack params -r rackName
```

## Additional Information

- **Message format:** each access line is sent with facility `user` and severity `info`, the syslog hostname `<rack>.system` and the tag `service/ingress-nginx/<pod>`, cut to 32 characters. The router's error lines carry the same hostname and tag. An access line is one whose message starts with `{`.
- **Fields:** each access line is a JSON object with the fields `time`, `remote_addr`, `x_forwarded_for`, `request_id`, `remote_user`, `bytes_sent`, `request_time`, `status`, `vhost`, `request_proto`, `path`, `request_query`, `request_length`, `duration`, `method`, `http_referrer` and `http_user_agent`.
- **Length:** over `tcp` and `tcp+tls`, a line of up to 16,384 bytes arrives whole. Over `udp`, a line that does not fit in the 1,024-byte message, header and tag included, arrives cut and is no longer valid JSON.
- **Volume:** the endpoint receives one line per request the public router serves, so size its quotas for the Rack's request rate.
- **Masking:** [`router_log_redact_params`](/configuration/rack-parameters/aws/router_log_redact_params) masks query parameter values in these lines before they are stored or forwarded.
- **Fluentd required:** the parameter has no effect while [fluentd_disable](/configuration/rack-parameters/aws/fluentd_disable) is `true`.
- **Contour:** on a Rack with [`router_type=contour`](/configuration/rack-parameters/aws/router_type), Convox does not collect Envoy's logs, so the parameter has no effect.
- **Validation:** boolean. The CLI rejects other values with `param 'syslog_access_logs' must be 'true' or 'false' (got "<value>")`.
- **Version:** requires Rack version `3.25.11` or later and CLI version `3.25.11` or later. An older CLI rejects the parameter with `unknown parameter 'syslog_access_logs' for aws provider`.
- **Downgrade:** downgrading below `3.25.11` removes the parameter with `NOTICE: removing parameters not supported by version <version>: syslog_access_logs` on stderr. A Rack managed through the Console keeps the stored value and applies it again on the next update to `3.25.11` or later. A self-managed Rack drops the value, so set it again after upgrading.
- **Providers:** AWS only. On GCP, Azure, DigitalOcean and metal Racks, every router line, access lines included, reaches `syslog` without this parameter.

## See Also

- [syslog](/configuration/rack-parameters/aws/syslog) for the endpoint and what Fluentd forwards
- [syslog_tls_verify](/configuration/rack-parameters/aws/syslog_tls_verify) for certificate verification on `tcp+tls` endpoints
- [router_log_redact_params](/configuration/rack-parameters/aws/router_log_redact_params) for masking query parameter values in router logs
- [access_log_retention_in_days](/configuration/rack-parameters/aws/access_log_retention_in_days) for the CloudWatch output of the same access logs
- [Logging](/configuration/logging#log-forwarding) for log forwarding on every provider
