---
title: "Logging"
slug: logging
url: /configuration/logging
---
# Logging

## How Logging Works

Convox automatically captures all output from your application's `stdout` and `stderr` streams. Logs from all processes across all services are aggregated and available through the CLI and Console.

In addition to application output, Convox also captures:

* State changes triggered by deployments
* Health check failures

Every log line is prefixed with a timestamp and a source identifier (e.g., `service/web/012345689`), making it easy to trace activity across services and processes.

## Viewing Logs

You can view logs for any application using the `convox logs` command. By default, this streams real-time logs from all services in the app.

### Basic Usage

```bash
$ convox logs -a myapp
2020-01-01T00:00:00Z service/web/012345689 starting on port 3000
2020-01-01T00:00:01Z service/web/012345689 GET / 200
2020-01-01T00:00:02Z service/web/012345689 GET /other 404
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
2020-01-01T00:00:00Z service/web/012345689 starting on port 3000
2020-01-01T00:00:01Z service/web/012345689 GET / 200
2020-01-01T00:00:02Z service/web/012345689 GET /other 404
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

## Log Retention

Convox streams logs in real-time and does not retain historical logs indefinitely. For long-term log storage and analysis, you should forward logs to an external service using the syslog integration described above, or use the built-in [Monitoring and Alerting](/configuration/monitoring) features.

## See Also

- [Monitoring and Alerting](/configuration/monitoring) for setting up monitoring
- [Datadog Integration](/integrations/monitoring) for forwarding logs to Datadog
