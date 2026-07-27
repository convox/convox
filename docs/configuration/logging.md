---
title: "Logging"
description: "Convox captures stdout and stderr from every Process across all Services, aggregating timestamped, source-tagged logs viewable through the CLI and Console."
slug: logging
url: /configuration/logging
---
# Logging

## How Logging Works

Convox automatically captures all output from your application's `stdout` and `stderr` streams. Logs from all processes across all services are aggregated and available through the CLI and Console.

In addition to application output, Convox also captures:

- State changes triggered by deployments
- Health check failures

Every log line is prefixed with a timestamp and a source identifier (e.g., `service/web/012345689`), making it easy to trace activity across services and processes.

## Viewing Logs

You can view logs for any application using the `convox logs` command. By default, this streams real-time logs from all services in the app.

### Basic Usage

```bash
$ convox logs -a myapp
2026-01-15T14:30:00Z service/web/012345689 starting on port 3000
2026-01-15T14:30:01Z service/web/012345689 GET / 200
2026-01-15T14:30:02Z service/web/012345689 GET /other 404
```

### Filtering by Service

Use the `--service` (or `-s`) flag to show logs from a specific service only:

```bash
$ convox logs -a myapp --service web
```

### Filtering by Content

Use the `--filter` flag to search for log lines containing a specific string:

```bash
$ convox logs -a myapp --filter "GET /api"
```

### Setting a Time Window

Use the `--since` flag to limit logs to a specific time window. Values can be expressed in minutes (`m`), hours (`h`), or days (`d`):

```bash
$ convox logs -a myapp --since 1h
```

### Viewing Historical Logs

By default, `convox logs` streams logs in real-time. Use the `--no-follow` flag to print historical logs and exit instead of continuing to stream:

```bash
$ convox logs -a myapp --since 20m --no-follow
2026-01-15T14:30:00Z service/web/012345689 starting on port 3000
2026-01-15T14:30:01Z service/web/012345689 GET / 200
2026-01-15T14:30:02Z service/web/012345689 GET /other 404
```

### Combining Flags

Flags can be combined for more targeted queries:

```bash
$ convox logs -a myapp --service web --filter "ERROR" --since 2h --no-follow
```

## Log Forwarding

Convox supports forwarding logs to external log aggregation services via syslog. To enable log forwarding, configure the `syslog` rack parameter with a syslog endpoint URL:

```bash
$ convox rack params set syslog=tcp+tls://logs.example.com:1234
```

This will forward all application and system logs to the specified syslog destination. See the [syslog rack parameter](/configuration/rack-parameters/aws/syslog) documentation for full configuration details.

Fluentd performs the forwarding, so `syslog` has no effect on a Rack with [fluentd_disable](/configuration/rack-parameters/aws/fluentd_disable) set to `true`. On an AWS Rack, Fluentd also writes container output to CloudWatch whenever it is running, and [cloudwatch_disable](/configuration/rack-parameters/aws/cloudwatch_disable) does not change that. Taking a Rack fully off CloudWatch therefore requires both `fluentd_disable=true` and `cloudwatch_disable=true`. In that configuration Convox does not forward logs anywhere: `convox logs --service <service>` still reads Pod logs directly, and any external collection has to come from an agent you run yourself.

## Fluentd Memory Tuning

Convox uses Fluentd as the log collector DaemonSet running on every node. The default memory allocation of `200Mi` is sufficient for most workloads, but racks with high log throughput may experience Fluentd OOM restarts and temporary log loss. You can tune the memory allocation with the `fluentd_memory` rack parameter:

```bash
$ convox rack params set fluentd_memory=512Mi -r rackName
```

See the provider-specific parameter pages for details:
- [fluentd_memory (AWS)](/configuration/rack-parameters/aws/fluentd_memory)
- [fluentd_memory (Azure)](/configuration/rack-parameters/azure/fluentd_memory)
- [fluentd_memory (GCP)](/configuration/rack-parameters/gcp/fluentd_memory)
- [fluentd_memory (DO)](/configuration/rack-parameters/do/fluentd_memory)

To disable Fluentd entirely (e.g., when using a custom logging solution), see [fluentd_disable](/configuration/rack-parameters/aws/fluentd_disable).

## CloudWatch on AWS Racks

On AWS, two writers put data into the same CloudWatch log group for an App, `/convox/<rack>/<app>`. Fluentd ships container output from every Pod, and the Rack controller writes Kubernetes events, deploy state transitions, and AWS resource provisioning messages. The Rack writes its own events to a separate group, `/convox/<rack>/system`, which is what `convox rack logs` reads.

This is why the whole-App view and the per-Service view show different content. `convox logs -a my-app` reads the CloudWatch group, so it returns container output and Rack-side event lines together. `convox logs -a my-app --service web` reads Pod logs directly through Kubernetes, so it returns container output only. CloudWatch applies the `--filter` pattern, which is why `--filter` narrows the whole-App view and is ignored when `--service` is set.

[cloudwatch_disable](/configuration/rack-parameters/aws/cloudwatch_disable) stops the Rack from creating, writing to, and reading its own CloudWatch log groups. While it is on, `convox logs -a my-app` and `convox rack logs` return empty output, and `convox logs -a my-app --service web` keeps working because it does not read CloudWatch. Fluentd is gated separately by [fluentd_disable](/configuration/rack-parameters/aws/fluentd_disable): with Fluentd still running, it continues to create the log groups and write container output into them.

## Log Retention

Convox streams logs in real-time and does not retain historical logs indefinitely. For long-term log storage and analysis, you should forward logs to an external service using the syslog integration described above, or use the built-in [Monitoring and Alerting](/configuration/monitoring) features.

## See Also

- [Monitoring and Alerting](/configuration/monitoring) for setting up monitoring
- [Datadog Integration](/integrations/monitoring) for forwarding logs to Datadog
