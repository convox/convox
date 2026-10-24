---
title: "logs"
description: "The convox logs command streams an app's logs continuously, with flags to filter by string or service, set a time window, and stop after current output."
slug: logs
url: /reference/cli/logs
---
# Logs

## Logs

Get logs for an app. By default, `convox logs` streams logs continuously. Use `--no-follow` to print current logs and exit.

With CLI 3.25.12 or later, a `--no-follow` read that ends before the end of its output prints what it received, then `ERROR: log stream ended before the end of output`, and exits 1, with or without `--service`. A complete read exits 0, so scripts can tell a complete read from a cut one.

### Usage
```bash
    convox logs
```
### Examples
```bash
    $ convox logs
    2026-03-18T12:47:41Z service/web/web-7cd6cc78dd-blvtn ns=template id=2bdd60aaf431 route=root at=end state=success elapsed=0.065
    2026-03-18T12:47:41Z service/web/web-7cd6cc78dd-blvtn ns=template id=2bdd60aaf431 route=root at=start method="GET" path="/" elapsed=0.029
    2026-03-18T12:47:43Z service/web/web-7cd6cc78dd-x2kq9 ns=template id=57c9464c88f6 route=root at=end state=success elapsed=0.070
    2026-03-18T12:47:43Z service/web/web-7cd6cc78dd-blvtn ns=template id=f5b0fcdd6f63 route=root at=start method="GET" path="/" elapsed=0.038
    ....

    $ convox logs --filter 2bdd60aaf431 --since 24h
    2026-03-18T12:47:41Z service/web/web-7cd6cc78dd-blvtn ns=template id=2bdd60aaf431 route=root at=end state=success elapsed=0.065
    2026-03-18T12:47:41Z service/web/web-7cd6cc78dd-blvtn ns=template id=2bdd60aaf431 route=root at=start method="GET" path="/" elapsed=0.029

    $ convox logs --tail 2 --since 1h --no-follow
    2026-03-18T12:47:43Z service/web/web-7cd6cc78dd-x2kq9 ns=template id=57c9464c88f6 route=root at=end state=success elapsed=0.070
    2026-03-18T12:47:43Z service/web/web-7cd6cc78dd-blvtn ns=template id=f5b0fcdd6f63 route=root at=start method="GET" path="/" elapsed=0.038
```

### Flags

| Flag | Short | Description |
| ---- | ----- | ----------- |
| `--filter` | | Without `--service`, show only matching lines. On AWS the value is a CloudWatch filter pattern; on GCP, Azure, DigitalOcean, metal and local Racks, a line matches when its message contains the text, case-sensitive (Rack version 3.25.12 or later). Ignored with `--service`. See [Filtering by Content](/configuration/logging#filtering-by-content) |
| `--no-follow` | | Print logs and exit rather than streaming |
| `--since` | | Time frame for log query (e.g., `24h`, `2m`; default `2m`) |
| `--service` | `-s` | Filter to a specific service |
| `--tail` | | With `--service`, the number of lines to read from the end of each Process's log. Without `--service`, the number of lines to print from the end of the App's `--since` window, from 0 to 10000 (Rack version 3.25.12 or later). See [Last Lines of the App](#last-lines-of-the-app) |
| `--allow-previous` | | With `--service`, include logs from previous container instances. Ignored without `--service` |
| `--max-log-requests` | | With `--service`, the maximum number of concurrent log follow streams (default `20`). Ignored without `--service` |

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

### Last Lines of the App

Without `--service`, `--tail <n>` prints the last `n` lines of the App's log in the `--since` window, counted after `--filter`, then the live stream when following. The count covers the whole App, not each Process. It requires Rack version 3.25.12 or later; earlier Racks print every line of the window. Local Racks ignore `--tail` without `--service`.

| Command | Returns |
| ------- | ------- |
| `convox logs --tail 20` | the last 20 lines from the last 2 minutes, then the live stream |
| `convox logs --tail 5 --since 1h --no-follow` | the last 5 lines from the last hour, then exits |
| `convox logs --filter 404 --tail 5 --since 1h --no-follow` | the last 5 matching lines from the last hour, then exits |
| `convox logs --tail 0` | no earlier lines, then the live stream |

- `n` is 0 to 10000.
- Nothing prints until the Rack has read the whole window, so a long `--since` on a busy App prints nothing for a while.

### Log Sources on AWS Racks

On an AWS Rack the whole-App view and the per-Service view come from different sources. `convox logs` without `--service` is served from the App's CloudWatch log group, while `convox logs --service <name>` reads Pod logs directly. CloudWatch applies the `--filter` pattern, so `--filter` narrows the whole-App view and is ignored when `--service` is set. On a Rack with [cloudwatch_disable](/configuration/rack-parameters/aws/cloudwatch_disable) set to `true`, the whole-App view returns empty output while `--service` continues to work.

A CloudWatch filter pattern is case-sensitive, and a term with characters other than letters and digits goes in double quotes, such as `--filter '"payment-failed"'`. See the [CloudWatch filter pattern syntax](https://docs.aws.amazon.com/AmazonCloudWatch/latest/logs/FilterAndPatternSyntax.html).

### Log Sources on GCP, Azure, DigitalOcean and Metal Racks

On GCP, Azure, DigitalOcean and metal Racks, `convox logs` without `--service` reads the Rack's Elasticsearch, and `convox logs --service <name>` reads Pod logs directly. From Rack version `3.25.11`, `--no-follow` prints every line in the `--since` window, oldest first. The live stream prints each line once. A line whose timestamp is up to 5 seconds older than the newest line already printed still prints when it arrives, so the live output is not strictly in timestamp order. A line more than 5 seconds older than that does not appear in the live stream; `--no-follow` shows it. See [Logging](/configuration/logging#elasticsearch-on-gcp-azure-digitalocean-and-metal-racks).

On Rack version 3.25.12 or later, `--filter` shows the lines whose message contains the text, case-sensitive and as plain text, so CloudWatch pattern syntax such as quotes or `?` is matched literally. `--tail` prints the last lines of the window, then the live stream when following. Lines still on their way to Elasticsearch when the window was read can print after them, even if they were written a few seconds before the command started; no line prints twice.

## See Also

- [Logging](/configuration/logging) for log configuration and forwarding
- [deploy-debug](/reference/cli/deploy-debug) for diagnosing pods that never reach a ready state
