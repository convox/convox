---
title: "Process Management"
description: "The convox cloud process commands exec into, list, inspect, and stop running processes, and run one-off commands in a new process, detached and waited on if needed."
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

Piped standard input is forwarded to the command, and the end of the pipe signals end of input:

```bash
$ printf 'hello\n' | convox cloud exec web-abc123 cat -a myapp -i production
hello
```

Output the command writes to standard error comes back with its standard output over the same connection, so lines from the two can interleave.

A Cloud machine is reached through the Console, so a command that reads standard input until end of input with nothing piped in never sees that end and waits until the session times out. Pipe at least one byte to release it, or start the command with `convox cloud run --detach --wait` instead. Both behaviors require machine version 3.25.5 or later.

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

A finished process adds an `Exit` row carrying its exit status. The row is absent while the process is still running, absent when no container ever started, and absent on machines before 3.25.5.

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
- `--id`: With `--detach`, put the process id alone on stdout and all other output on stderr
- `--release`: Specific release
- `--retain`: With `--detach`, seconds to keep the finished process readable by `ps info`
- `--timeout`: Seconds before an attached run is abandoned, or before `--wait` gives up (default `3600`)
- `--wait`: With `--detach`, wait for the process to finish and exit with its status

**Example:**
```bash
$ convox cloud run web "rake db:migrate" -a myapp -i production
Running... OK

$ convox cloud run web bash -a myapp -i production
/app #
```

`--wait` reads the process record every 5 seconds until it reports `complete` or `failed`, then exits with the process's own exit code, so a pipeline step fails when the command does:

```bash
$ convox cloud run web "bin/migrate" --detach --wait -a myapp -i production
Running detached process... OK, web-s43xf
```

`--retain` keeps the finished process readable by `convox cloud ps info` after its command exits. The machine caps retention at 600 seconds, and `--wait` raises it to 60 seconds when the run asks for less. Retention is best effort, so a process the pipeline can no longer find is an unknown outcome and the step must treat it as a failure.

```bash
$ pid=$(convox cloud run web "bin/backfill" --detach --id --retain 600 -a myapp -i production)
$ convox cloud ps info "$pid" -a myapp -i production
```

`--wait` and `--retain` require machine version 3.25.5 or later. `--id` is handled by the CLI and works against any version. See [run](/reference/cli/run#detached-runs) for the full detached-run reference.
