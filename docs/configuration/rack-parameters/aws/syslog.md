---
title: "syslog"
description: "The syslog AWS rack parameter sets the endpoint that rack logs are forwarded to, such as tcp+tls://example.org:1234, defaulting to empty (no forwarding)."
slug: syslog
url: /configuration/rack-parameters/aws/syslog
---

# syslog

## Description
The `syslog` parameter sets the endpoint Fluentd forwards logs to, for example `tcp+tls://example.org:1234`. The URL scheme is `tcp`, `tcp+tls` or `udp`, and the port defaults to `514` when the URL leaves it out.

Fluentd forwards the container output of every App's Services, Timers, `convox run` Processes and Builds, and of the Rack's own system Pods such as the API, router and resolver, over one connection from each node's Fluentd pod. These lines are not forwarded:

- Nginx access log lines from the `ingress-nginx` router, which go only to the `/nginx-access-logs` stream in `/convox/<rack>/system`.
- Event lines the Rack writes to CloudWatch itself, such as deploy state changes, Kubernetes events and resource provisioning messages.
- Output of Pods Fluentd does not collect, which is not in CloudWatch either:
  - containerized Resources such as `postgres` or `redis`
  - Contour and Envoy on a Rack with `router_type=contour`
  - Kubernetes add-ons such as CoreDNS or the Cluster Autoscaler
  - Builds on a Rack with [`pod_security_mode=enforce`](/configuration/rack-parameters/aws/pod_security_mode), which run in a separate namespace; `convox builds logs` still shows their output
  - a Service whose name starts with `fluentd` or `cloudwatch-agent`, and its `convox run` Processes
- Lines lost after an endpoint failure (see below).

## Default Value
The default value for `syslog` is an empty string. When set to an empty string, syslog forwarding is not enabled.

## Use Cases
- **Centralized Logging**: Forward logs to a centralized syslog server for better log management and analysis.
- **Compliance and Auditing**: Ensure that logs are forwarded to a secure and centralized location to meet compliance and auditing requirements.

## Setting Parameters
To set the `syslog` parameter, use the following command:
```bash
$ convox rack params set syslog='tcp+tls://example.org:1234' -r rackName
Updating parameters... OK
```
The change rolls the Fluentd pods on every node. Forwarding to CloudWatch continues throughout.

## Additional Information
- **Message format:** each line is sent with facility `user` and severity `info`. The syslog hostname is `<rack>.<app>` and the tag is `<type>/<name>/<pod>`, cut to 32 characters: `service/web/<pod>` for a Service, `timer/<timer>/<pod>` for a Timer, and `process/<service>/<pod>` for a `convox run` Process. Each message, header and tag included, is cut at 1024 bytes.
- **TLS:** Fluentd verifies a `tcp+tls` endpoint's certificate against public certificate authorities. For an endpoint with a self-signed or private-CA certificate, set [`syslog_tls_verify`](/configuration/rack-parameters/aws/syslog_tls_verify) to `false`.
- **Failing endpoint:** when a `tcp` or `tcp+tls` endpoint refuses the connection, is unreachable or stops reading, Fluentd drops the line it was sending, logs a warning, skips syslog output for 60 seconds, then reconnects. Lines written during the pause are not resent. Each failure delays CloudWatch delivery on that node by up to about 10 seconds, and nothing is lost from CloudWatch, except App lines while [app_cloudwatch_disable](/configuration/rack-parameters/aws/app_cloudwatch_disable) is `true`, which then have no other destination. A `udp` endpoint that is down returns no error, so lines sent to it are lost without a warning.
- **CloudWatch:** forwarding is a copy. Fluentd still writes to CloudWatch, and [app_cloudwatch_disable](/configuration/rack-parameters/aws/app_cloudwatch_disable) takes App logs off CloudWatch without stopping forwarding.
- **Fluentd required:** `syslog` has no effect while [fluentd_disable](/configuration/rack-parameters/aws/fluentd_disable) is `true`.
- **Volume:** the endpoint receives the full log volume of every App on the Rack, so size its quotas for it.

## See Also
- [syslog_tls_verify](/configuration/rack-parameters/aws/syslog_tls_verify) for certificate verification on `tcp+tls` endpoints
- [Logging](/configuration/logging#log-forwarding) for log forwarding on every provider
- [fluentd_memory](/configuration/rack-parameters/aws/fluentd_memory) for Fluentd memory on high log volume
