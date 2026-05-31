---
title: "Process Management"
description: "The convox cloud process commands exec into, list, inspect, and stop running processes, and run one-off commands in a new process."
slug: processes
url: /cloud/cli-reference/processes
---

# Process Management

### exec

Execute a command in a running process.

```bash
$ convox cloud exec <pid> <command> -a <app> -i <machine>
```

**Example:**
```bash
$ convox cloud exec web-abc123 bash -a myapp -i production
/app #
```

### ps

List running processes.

```bash
$ convox cloud ps -a <app> -i <machine>
```

**Options:**
- `--release`: Specific release
- `--service`: Filter by service
- `--watch`: Watch for updates

**Example:**
```bash
$ convox cloud ps -a myapp -i production
ID            SERVICE  STATUS   RELEASE      STARTED     COMMAND
web-abc123    web      running  RABCDEFGHI   1 hour ago  npm start
worker-def456 worker   running  RABCDEFGHI   1 hour ago  npm run worker
```

### ps info

Get information about a specific process.

```bash
$ convox cloud ps info <pid> -a <app> -i <machine>
```

### ps stop

Stop a running process.

```bash
$ convox cloud ps stop <pid> -a <app> -i <machine>
```

**Example:**
```bash
$ convox cloud ps stop web-abc123 -a myapp -i production
Stopping web-abc123... OK
```

### run

Run a one-off command in a new process.

```bash
$ convox cloud run <service> <command> -a <app> -i <machine>
```

**Options:**
- `--cpu`: CPU allocation (millicores)
- `--memory`: Memory allocation (MB)
- `--detach`: Run in background
- `--entrypoint`: Override entrypoint
- `--release`: Specific release

**Example:**
```bash
$ convox cloud run web "rake db:migrate" -a myapp -i production
Running... OK

$ convox cloud run web bash -a myapp -i production
/app #
```
