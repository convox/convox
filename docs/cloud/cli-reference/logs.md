---
title: "Monitoring Commands"
description: "The convox cloud logs command streams a Service's logs, with flags to set a time window, tail lines, or dump logs without following."
slug: logs
url: /cloud/cli-reference/logs
---

# Monitoring Commands

### logs

Stream logs from a Service of an application.

```bash
$ convox cloud logs -a <app> -i <machine> --service <service>
```

`--service` is required on a Cloud machine. Without it, the command fails with `logs must specify --service`.

**Options:**
- `--allow-previous`: Include previous container logs
- `--filter`: Has no effect on a Cloud machine, because `--service` is required and `--filter` is ignored with `--service`
- `--no-follow`: Don't stream logs
- `--service`: Service to read logs from (required)
- `--since`: Time window (e.g., "2h", "30m"; default "2m")
- `--tail`: Number of lines to read from the end of each Process's log

**Example:**
```bash
$ convox cloud logs -a myapp -i production --service web --since 1h
[pod/web-7cd6cc78dd-blvtn/myapp] 2026-01-15T10:30:00.123456789Z GET / 200
[pod/web-7cd6cc78dd-blvtn/myapp] 2026-01-15T10:30:15.482913004Z GET /api/users 200
```

Each line starts with the Process that wrote it, `[pod/<process>/<app>]`, and its timestamp. See [logs](/reference/cli/logs#service-logs) for how a Service's logs are read.
