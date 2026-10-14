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

On Rack version `3.25.11` or later, `--service` returns every line each Process wrote in the `--since` window. Earlier Racks return the last 10 lines of each Process unless `--tail` is set. `--tail <n>` limits the output to the last `n` lines of each Process. See [logs](/reference/cli/logs#service-logs).

### Filtering by Content

Use the `--filter` flag to search for log lines containing a specific string:

```bash
$ convox logs -a myapp --filter "GET /api"
```

### Setting a Time Window

Use the `--since` flag to limit logs to a specific time window. Values are durations in seconds (`s`), minutes (`m`) or hours (`h`), such as `90s`, `20m` or `48h`. There is no day unit, so one day is `24h`. Without `--since`, the window is the last 2 minutes:

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
$ convox logs -a myapp --service web --since 2h --no-follow
```

`--filter` is ignored when `--service` is set.

## Log Forwarding

Convox supports forwarding logs to external log aggregation services via syslog. To enable log forwarding, configure the `syslog` rack parameter with a syslog endpoint URL:

```bash
$ convox rack params set syslog=tcp+tls://logs.example.com:1234
```

Fluentd forwards the container output of every App's Services, Timers, `convox run` Processes and Builds, and of the Rack's own system Pods such as the API, router and resolver, one syslog message per line. The URL scheme is `tcp`, `tcp+tls` or `udp`; any other scheme forwards nothing. Over `tcp` and `tcp+tls`, a line of up to 16,384 bytes arrives whole. Over `udp`, each message, header and tag included, is cut at 1,024 bytes. The syslog tag, `<type>/<name>/<pod>` such as `service/web/<pod>`, is cut at 32 characters. A receiver with a smaller limit, such as rsyslog's default of 8 KiB, cuts or splits long lines on its side.

These lines never reach the endpoint:

| Lines | Why |
|---|---|
| Containerized Resources such as `postgres` or `redis`, Contour and Envoy on a Rack with `router_type=contour`, Kubernetes add-ons such as CoreDNS or the Cluster Autoscaler, and a Service whose name starts with `fluentd` or `cloudwatch-agent` | Fluentd does not collect them, on any provider, so they are not in CloudWatch or Elasticsearch either |
| Builds on an AWS Rack with [`pod_security_mode=enforce`](/configuration/rack-parameters/aws/pod_security_mode) | they run in a separate namespace Fluentd does not collect from; `convox builds logs` still shows their output |
| Deploy state changes, Kubernetes events and resource provisioning messages | the Rack writes them to CloudWatch or Elasticsearch itself |
| The public `ingress-nginx` router's access logs, on AWS, unless [`syslog_access_logs`](/configuration/rack-parameters/aws/syslog_access_logs) is `true` | they go only to the `/nginx-access-logs` stream of `/convox/<rack>/system`. The internal router's access logs are forwarded either way |
| Lines written while syslog output is paused for 60 seconds after a `tcp` or `tcp+tls` endpoint failure | they are not resent; CloudWatch or Elasticsearch still receives them, except App lines on AWS while `app_cloudwatch_disable` is `true` |
| Lines sent to a `udp` endpoint that is down | UDP returns no error, so they are lost without a warning |

A `tcp+tls` endpoint must present a certificate that verifies against public certificate authorities, unless `syslog_tls_verify` is set to `false` for a self-signed or private-CA endpoint ([AWS](/configuration/rack-parameters/aws/syslog_tls_verify), [GCP](/configuration/rack-parameters/gcp/syslog_tls_verify), [Azure](/configuration/rack-parameters/azure/syslog_tls_verify), [DigitalOcean](/configuration/rack-parameters/do/syslog_tls_verify)). AWS Racks verify from Rack version `3.25.10`, and GCP, Azure, DigitalOcean and metal Racks from `3.25.11`. On those Racks, forwarding to a self-signed or private-CA endpoint stops when the Rack updates to `3.25.11`, until `syslog_tls_verify=false` is set with a `3.25.11` CLI.

`router_log_redact_params` masks query parameter values, such as a token in a password reset link, in the router's access and error lines before Fluentd stores or forwards them ([AWS](/configuration/rack-parameters/aws/router_log_redact_params), [GCP](/configuration/rack-parameters/gcp/router_log_redact_params), [Azure](/configuration/rack-parameters/azure/router_log_redact_params), [DigitalOcean](/configuration/rack-parameters/do/router_log_redact_params)).

See the `syslog` parameter page for each provider: [AWS](/configuration/rack-parameters/aws/syslog), [GCP](/configuration/rack-parameters/gcp/syslog), [Azure](/configuration/rack-parameters/azure/syslog), [DigitalOcean](/configuration/rack-parameters/do/syslog).

Fluentd performs the forwarding, so `syslog` has no effect on a Rack with [fluentd_disable](/configuration/rack-parameters/aws/fluentd_disable) set to `true`. On an AWS Rack, Fluentd also writes container output to CloudWatch whenever it is running, and [cloudwatch_disable](/configuration/rack-parameters/aws/cloudwatch_disable) does not change that. Taking a Rack fully off CloudWatch therefore requires both `fluentd_disable=true` and `cloudwatch_disable=true`. In that configuration Convox does not forward logs anywhere: `convox logs --service <service>` still reads Pod logs directly, and any external collection has to come from an agent you run yourself.

[app_cloudwatch_disable](/configuration/rack-parameters/aws/app_cloudwatch_disable) is the middle option. It takes App logs off CloudWatch while Fluentd keeps running, so syslog forwarding of App output continues and `convox rack logs` still works.

## Fluentd Memory Tuning

Convox runs Fluentd as a DaemonSet on every node at the `system-node-critical` priority, with its memory request and limit both set to `fluentd_memory` (default `200Mi`). On a node with no room for it, the scheduler evicts lower-priority Pods to place it.

Fluentd holds lines it has not yet delivered in a buffer sized from `fluentd_memory`. When CloudWatch or Elasticsearch is slow or unreachable long enough to fill it, Fluentd stops reading container logs until there is room, and syslog forwarding pauses with it. The lines arrive late rather than being lost, unless the node rotates a container's log file before Fluentd reads it. A larger `fluentd_memory` lets Fluentd absorb a longer outage before it pauses:

```bash
$ convox rack params set fluentd_memory=512Mi -r rackName
```

See the provider-specific parameter pages for details:
- [fluentd_memory (AWS)](/configuration/rack-parameters/aws/fluentd_memory)
- [fluentd_memory (Azure)](/configuration/rack-parameters/azure/fluentd_memory)
- [fluentd_memory (GCP)](/configuration/rack-parameters/gcp/fluentd_memory)
- [fluentd_memory (DO)](/configuration/rack-parameters/do/fluentd_memory)

Values below `200Mi` leave Fluentd little room under load. On GCP, Azure, DigitalOcean and metal, the Fluentd in Rack version `3.25.11` uses about 26 MiB more memory at idle than earlier versions, so raise `fluentd_memory` to at least `200Mi` before updating a Rack that runs below it.

On an AWS Rack, [fluentd_disable](/configuration/rack-parameters/aws/fluentd_disable) turns Fluentd off entirely, for example when you run your own log collector.

## CloudWatch on AWS Racks

On AWS, two writers put data into the same CloudWatch log group for an App, `/convox/<rack>/<app>`. Fluentd ships the container output of the App's Services, Timers, Processes and Builds (see [Log Forwarding](#log-forwarding) for the Pods it does not collect), and the Rack controller writes Kubernetes events, deploy state transitions, and AWS resource provisioning messages. The Rack writes its own events to a separate group, `/convox/<rack>/system`, which is what `convox rack logs` reads. Fluentd writes the container output of Rack system Pods to the same group, including the router's Nginx error log and controller output in the `service/ingress-nginx/<pod>` streams and Nginx access logs in the `/nginx-access-logs` stream. To reduce the warning-level Nginx lines, add `error-log-level=error` to [nginx_additional_config](/configuration/rack-parameters/aws/nginx_additional_config).

This is why the whole-App view and the per-Service view show different content. `convox logs -a my-app` reads the CloudWatch group, so it returns container output and Rack-side event lines together. `convox logs -a my-app --service web` reads Pod logs directly through Kubernetes, so it returns container output only. CloudWatch applies the `--filter` pattern, which is why `--filter` narrows the whole-App view and is ignored when `--service` is set.

[cloudwatch_disable](/configuration/rack-parameters/aws/cloudwatch_disable) stops the Rack from creating, writing to, and reading its own CloudWatch log groups. While it is on, `convox logs -a my-app` and `convox rack logs` return empty output, and `convox logs -a my-app --service web` keeps working because it does not read CloudWatch. Fluentd is gated separately by [fluentd_disable](/configuration/rack-parameters/aws/fluentd_disable): with Fluentd still running, it continues to create the log groups and write container output into them.

[app_cloudwatch_disable](/configuration/rack-parameters/aws/app_cloudwatch_disable) cuts across the same two writers along the other axis. It stops both of them on the per-App groups and exempts `/convox/<rack>/system`, so the App groups stop being created and filled while `convox rack logs` keeps returning Rack events, Rack system Pod output, and Nginx access logs. Neither parameter is a subset of the other: `cloudwatch_disable` stops one writer on both groups, `app_cloudwatch_disable` stops both writers on one group.

### Event Lines for Apps With a Hyphen in the Name

Before Rack version `3.25.5`, the Rack controller wrote Kubernetes events and deploy state transitions for an App whose name contains a hyphen into a group named after the first segment of that name, `/convox/<rack>/my-my-app` for an App called `my-app`. Container output from Fluentd always went to the correct group, so the App's own group was missing only the Rack-side lines.

Updating a Rack to `3.25.5` sends those lines to the App's own group, and `convox logs -a my-app` shows them from that point on. The group created under the wrong name keeps whatever it already holds. Convox never deletes log groups, so remove it in the CloudWatch console once you no longer need its contents.

## Elasticsearch on GCP, Azure, DigitalOcean and Metal Racks

On GCP, Azure, DigitalOcean and metal Racks, logs are stored in an Elasticsearch that runs inside the Rack. Fluentd writes the container output of every App's Services, Timers, Processes and Builds, and of the Rack's system Pods, to it (see [Log Forwarding](#log-forwarding) for the Pods it does not collect), and the Rack writes Kubernetes events and deploy state changes itself. Elasticsearch runs as two Pods, or one on a DigitalOcean Rack with `high_availability` set to `false`, each with its own persistent volume sized by `elasticsearch_disk` ([GCP](/configuration/rack-parameters/gcp/elasticsearch_disk), [Azure](/configuration/rack-parameters/azure/elasticsearch_disk), [DigitalOcean](/configuration/rack-parameters/do/elasticsearch_disk)).

`convox logs -a my-app` and `convox rack logs` read Elasticsearch. `convox logs -a my-app --service web` reads Pod logs directly through Kubernetes, so it returns container output only and keeps working while Elasticsearch is unavailable.

From Rack version `3.25.11`, reads from Elasticsearch behave as follows:

| Case | Behavior |
|---|---|
| `--no-follow` | prints every line from the start of the `--since` window up to the moment the command started, oldest first, then exits |
| Live stream | prints each line once. A line whose timestamp is up to 5 seconds older than the newest line already printed still prints when it arrives, so the output is not strictly in timestamp order |
| A line more than 5 seconds older than the newest line already printed | does not appear in the live stream; `--no-follow` shows it |

`--tail` applies only with `--service`.

From Rack version `3.25.11`, lines Fluentd has read but not yet written to Elasticsearch wait on each node's disk in `/var/log/fluentd-buffers/elasticsearch`, readable by root only, and the next Fluentd Pod sends them. A Fluentd restart or a Rack update does not drop them, but can show a second or two of lines twice. After a downgrade below `3.25.11`, lines still waiting there arrive after the next update to `3.25.11` or later.

While Elasticsearch refuses writes, for example after its disk fills, Fluentd retries about every 30 seconds, so delivery resumes within about 30 seconds of Elasticsearch accepting writes again. If Fluentd's buffer fills first, it pauses reading container logs, and syslog forwarding pauses with it. Fluentd reconnects after an error, so it recovers on its own when Elasticsearch Pods are replaced.

On a metal Rack whose storage class does not allow volume expansion, raising `elasticsearch_disk` leaves the volumes at their size, and `convox rack logs` shows the refusal as an `ns=elastic at=expand` line with an `error=` field.

## Log Retention

On an AWS Rack, how long logs survive is a property of each CloudWatch log group. A log group Convox creates starts at 7 days while `cloudwatch_retention_in_days` is unset, and follows that parameter once it is set. A log group Fluentd creates has no retention policy, so it never expires, and neither does the EKS control plane log group.

[cloudwatch_retention_in_days](/configuration/rack-parameters/aws/cloudwatch_retention_in_days) sets a single retention period across the Rack system log group, every App log group, and the EKS control plane log group, or `Never` to stop them expiring. Left unset, Convox does not manage retention and every group keeps the value it already has. It requires Rack version `3.25.7` or later. An App overrides it for its own log group with [appSettings.awsLogs](/configuration/app-settings).

Setting `cloudwatch_retention_in_days`, or lowering it, deletes log data. CloudWatch applies the retention window to the data already in a group, so everything older than it is gone within hours, and raising the value later does not bring it back. A group that never expired until now, which is every group Fluentd created and the EKS control plane group, loses everything older than the window the first time you set a period.

On GCP, Azure, DigitalOcean and metal Racks, logs are kept in the Rack's Elasticsearch, so how much history a Rack keeps depends on how much it logs and on `elasticsearch_disk` ([GCP](/configuration/rack-parameters/gcp/elasticsearch_disk), [Azure](/configuration/rack-parameters/azure/elasticsearch_disk), [DigitalOcean](/configuration/rack-parameters/do/elasticsearch_disk)).

From Rack version `3.25.11`, the Rack checks Elasticsearch disk use every minute, and while any volume is 80% full or more it deletes the oldest day of logs, one day per check, until every volume is back under that mark. It never deletes the current day (UTC). A day of logs written on `3.25.11` or later is deleted once it is more than 365 days old, regardless of disk use. Logs written before the update are deleted only to free disk space. Each deletion writes an `ns=elastic at=retain` line to `convox rack logs`.

Logs written before the Rack updated to `3.25.11` are deleted first, before any day of newer logs, one App's whole pre-update history per check, largest first. On a Rack that updates with a volume already past the mark, these deletions start right after the update.

If Elasticsearch stopped accepting logs because its disk filled, the Rack lifts that block once there is room again, and logging resumes. When the current day's logs alone fill a volume, nothing can be deleted until the next UTC day, and the Rack writes `warning="only today's logs are left to delete"` to `convox rack logs` once an hour. Raise `elasticsearch_disk` on a Rack that reaches this point.

For long-term storage and analysis beyond CloudWatch or the Rack's Elasticsearch, forward logs to an external service with the syslog integration described above, or use [Monitoring and Alerting](/configuration/monitoring).

## See Also

- [Monitoring and Alerting](/configuration/monitoring) for setting up monitoring
- [Datadog Integration](/integrations/monitoring) for forwarding logs to Datadog
- [cloudwatch_retention_in_days](/configuration/rack-parameters/aws/cloudwatch_retention_in_days) for Rack-wide CloudWatch log retention
- [syslog](/configuration/rack-parameters/aws/syslog) and [syslog_tls_verify](/configuration/rack-parameters/aws/syslog_tls_verify) for forwarding logs from an AWS Rack
- [syslog_access_logs](/configuration/rack-parameters/aws/syslog_access_logs) for forwarding public router access logs from an AWS Rack
- [router_log_redact_params](/configuration/rack-parameters/aws/router_log_redact_params) for masking query parameter values in router logs
- [elasticsearch_disk](/configuration/rack-parameters/gcp/elasticsearch_disk) ([Azure](/configuration/rack-parameters/azure/elasticsearch_disk), [DigitalOcean](/configuration/rack-parameters/do/elasticsearch_disk)) for the size of the Elasticsearch log volumes
