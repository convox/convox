---
title: "Monitoring Commands"
slug: logs
url: /cloud/cli-reference/logs
---

# Monitoring Commands

### logs

Stream logs from an application.

```bash
$ convox cloud logs -a <app> -i <machine>
```

**Options:**
- `--allow-previous`: Include previous container logs
- `--filter`: Filter log output
- `--no-follow`: Don't stream logs
- `--service`: Specific service
- `--since`: Time window (e.g., "2h", "30m")
- `--tail`: Number of lines to show

**Example:**
```bash
$ convox cloud logs -a myapp -i production --service web --since 1h
2026-01-15T10:30:00Z service/web/abc123 GET / 200
2026-01-15T10:30:15Z service/web/abc123 GET /api/users 200
```
