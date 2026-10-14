---
title: "logs"
description: "The convox logs command streams an app's logs continuously, with flags to filter by string or service, set a time window, and stop after current output."
slug: logs
url: /reference/cli/logs
---
# Logs

## Logs

Get logs for an app. By default, `convox logs` streams logs continuously. Use `--no-follow` to print current logs and exit.

### Usage
```bash
    convox logs
```
### Examples
```bash
    $ convox logs
    2026-03-18T12:47:41Z service/web/77f0e67e-4886-4aa8-be56-1d19a3aab53b ns=template id=2bdd60aaf431 route=root at=end state=success elapsed=0.065
    2026-03-18T12:47:41Z service/web/77f0e67e-4886-4aa8-be56-1d19a3aab53b ns=template id=2bdd60aaf431 route=root at=start method="GET" path="/" elapsed=0.029
    2026-03-18T12:47:43Z service/web/a81ba08c-6dbe-48a4-88e6-da5f940156ae ns=template id=57c9464c88f6 route=root at=end state=success elapsed=0.070
    2026-03-18T12:47:43Z service/web/77f0e67e-4886-4aa8-be56-1d19a3aab53b ns=template id=f5b0fcdd6f63 route=root at=start method="GET" path="/" elapsed=0.038
    ....

    $ convox logs --filter 2bdd60aaf431 --since 24h
    2026-03-18T12:47:41Z service/web/77f0e67e-4886-4aa8-be56-1d19a3aab53b ns=template id=2bdd60aaf431 route=root at=end state=success elapsed=0.065
    2026-03-18T12:47:41Z service/web/77f0e67e-4886-4aa8-be56-1d19a3aab53b ns=template id=2bdd60aaf431 route=root at=start method="GET" path="/" elapsed=0.029
```

### Flags

| Flag | Short | Description |
| ---- | ----- | ----------- |
| `--filter` | | Filter for a specific string within the logs |
| `--no-follow` | | Print logs and exit rather than streaming |
| `--since` | | Time frame for log query (e.g., `24h`, `2m`; default `2m`) |
| `--service` | `-s` | Filter to a specific service |
| `--tail` | | With `--service`, the number of lines to read from the end of each Process's log. Ignored without `--service` |
| `--allow-previous` | | Include logs from previous container instances |
| `--max-log-requests` | | Maximum number of concurrent log follow streams (default `20`) |

### Service Logs

`convox logs --service <name>` reads each of the Service's Processes directly from Kubernetes. On Rack version `3.25.11` or later it returns every line each Process wrote in the `--since` window (default `2m`), in follow and `--no-follow` mode. Earlier Racks return the last 10 lines of each Process unless `--tail` is set.

| Command | Returns |
| ------- | ------- |
| `convox logs -s web` | each Process's lines from the last 2 minutes, then the live stream |
| `convox logs -s web --since 2h --no-follow` | each Process's lines from the last 2 hours, then exits |
| `convox logs -s web --tail 10` | the last 10 lines of each Process within the last 2 minutes, then the live stream |
| `convox logs -s web --since 0s --no-follow` | each Process's whole current log file, then exits |

```bash
    $ convox logs -s web --since 2h --no-follow
    [pod/web-7cd6cc78dd-blvtn/myapp] 2026-03-18T12:47:41.364333569Z GET / 200
    [pod/web-7cd6cc78dd-blvtn/myapp] 2026-03-18T12:47:43.108422907Z GET /other 404
    [pod/web-7cd6cc78dd-x2kq9/myapp] 2026-03-18T12:47:42.920115381Z GET / 200
    [pod/web-7cd6cc78dd-x2kq9/myapp] 2026-03-18T12:47:44.511290637Z GET /health 200
```

- Each line starts with `[pod/<process>/<app>]` and its timestamp, unlike the `service/<service>/<process>` prefix of the whole-App view.
- The read covers the current log file of the Service's current Processes. A Process replaced inside the window is absent, and log rotation on the node can drop the oldest part of a long window.
- With `--no-follow` and more than one Process, each Process's lines print as one block, not merged by time. In follow mode the streams interleave.
- `--since 0s` reads the whole log file only with `--service`. Without `--service`, `0s` starts the read at the current time and returns no earlier lines.
- `--filter` is ignored with `--service`.

### Tailing High-Concurrency Services

`convox logs --service <name>` streams logs from every pod that matches the selector. The underlying kubectl plumbing caps concurrency at 20 follow streams by default. Following the logs of a Service that runs more than 20 Processes fails with:

```text
ERROR: you are attempting to follow 50 log streams, but maximum allowed concurrency is 20, use --max-log-requests to increase the limit
```

A `--no-follow` read has no cap and reads each Process in turn. Pass `--max-log-requests N` to raise the concurrency cap when following.

```bash
    $ convox logs --service pii-worker --since 5m --max-log-requests 50
```

Set the value to at least the number of Processes the Service runs. Log streams are persistent HTTP connections, so very large values put sustained load on the API server.

### Log Sources on AWS Racks

On an AWS Rack the whole-App view and the per-Service view come from different sources. `convox logs` without `--service` is served from the App's CloudWatch log group, while `convox logs --service <name>` reads Pod logs directly. CloudWatch applies the `--filter` pattern, so `--filter` narrows the whole-App view and is ignored when `--service` is set. On a Rack with [cloudwatch_disable](/configuration/rack-parameters/aws/cloudwatch_disable) set to `true`, the whole-App view returns empty output while `--service` continues to work.

### Log Sources on GCP, Azure, DigitalOcean and Metal Racks

On GCP, Azure, DigitalOcean and metal Racks, `convox logs` without `--service` reads the Rack's Elasticsearch, and `convox logs --service <name>` reads Pod logs directly. From Rack version `3.25.11`, `--no-follow` prints every line in the `--since` window, oldest first. The live stream prints each line once. A line whose timestamp is up to 5 seconds older than the newest line already printed still prints when it arrives, so the live output is not strictly in timestamp order. A line more than 5 seconds older than that does not appear in the live stream; `--no-follow` shows it. `--tail` applies only with `--service`. See [Logging](/configuration/logging#elasticsearch-on-gcp-azure-digitalocean-and-metal-racks).

## See Also

- [Logging](/configuration/logging) for log configuration and forwarding
- [deploy-debug](/reference/cli/deploy-debug) for diagnosing pods that never reach a ready state
