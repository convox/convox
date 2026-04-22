---
title: "logs"
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
| `--since` | | Time frame for log query (e.g., `24h`, `2m`) |
| `--service` | `-s` | Filter to a specific service |
| `--tail` | | Number of lines to tail (service-specific logging only) |
| `--allow-previous` | | Include logs from previous container instances |
| `--max-log-requests` | | Maximum number of concurrent log follow streams (default `20`) |

### Tailing High-Concurrency Services

`convox logs --service <name>` streams logs from every pod that matches the selector. The underlying kubectl plumbing caps concurrency at 20 follow streams by default. When a service runs with more than 20 pods, the command fails with:

```text
ERROR: you are attempting to follow 50 log streams, but maximum allowed concurrency is 20, use --max-log-requests to increase the limit
```

Pass `--max-log-requests N` to raise the concurrency cap. The flag is also wired into `convox rack logs` for rack-level system log streams.

```bash
    $ convox logs --service pii-worker --since 5m --max-log-requests 50
```

Set the value to at least the pod count of the target service. Log streams are persistent HTTP connections, so very large values put sustained load on the API server — prefer filtering by `--service` and a modest concurrency over tailing the full app stream without the flag.

## See Also

- [Logging](/configuration/logging) for log configuration and forwarding
- [deploy-debug](/reference/cli/deploy-debug) for diagnosing pods that never reach a ready state
