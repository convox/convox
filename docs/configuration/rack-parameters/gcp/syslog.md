---
title: "syslog"
description: "The syslog GCP rack parameter sets the endpoint to forward rack logs to a syslog server, such as tcp+tls://example.org:1234, disabled by default."
slug: syslog
url: /configuration/rack-parameters/gcp/syslog
---

# syslog

## Description
The `syslog` parameter specifies the endpoint to forward logs to a syslog server (e.g. **tcp+tls://example.org:1234**).

## Default Value
The default value for `syslog` is `""`.

## Use Cases
- **Centralized Logging**: Forward logs to a central syslog server for easier monitoring and analysis.
- **Compliance**: Meet compliance requirements by ensuring all logs are collected and stored in a central location.

## Setting Parameters
To set the `syslog` parameter, use the following command:
```bash
$ convox rack params set syslog=tcp+tls://example.org:1234 -r rackName
Updating parameters... OK
```
This command sets the `syslog` parameter to the specified value.

## Additional Information
When set to `""`, syslog forwarding is not enabled. This parameter is optional and can be configured based on your specific logging needs. Ensure that the syslog endpoint is reachable and properly configured to receive logs from your Convox rack.

- **Scheme and port:** the URL scheme is `tcp`, `tcp+tls` or `udp`, and the port defaults to `514` when the URL leaves it out.
- **What is forwarded:** Fluentd forwards the container output of every App's Services, Timers, `convox run` Processes and Builds, and of the Rack's own system Pods such as the API, router and resolver. It does not collect containerized Resources such as `postgres` or `redis`, Kubernetes add-ons such as CoreDNS, or a Service whose name starts with `fluentd` or `cloudwatch-agent`. The Rack's own event lines, such as deploy state changes, are written to Elasticsearch directly and are not forwarded.
- **Message format:** each line is sent with facility `user` and severity `info`. The syslog hostname is `<rack>.<app>` and the tag is `<type>/<name>/<pod>`, cut to 32 characters: `service/web/<pod>` for a Service, `timer/<timer>/<pod>` for a Timer, and `process/<service>/<pod>` for a `convox run` Process. Each message, header and tag included, is cut at 1024 bytes.
- **`udp` endpoints:** a `udp` endpoint that is down returns no error, so lines sent to it are lost without a warning.
