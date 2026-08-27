---
title: "exec"
description: "The convox exec command runs a command inside an existing running process, for example opening a shell in a container to inspect or debug it, piping input to it, or reading its error output."
slug: exec
url: /reference/cli/exec
---
# exec

## exec

Execute a command in a running process

### Usage
```bash
    convox exec <pid> <command>
```
### Examples
```bash
    $ convox exec 7b6bccfd9fdf bash
    bash-3.2$
```

## Piped Input

`convox exec` forwards piped standard input to the command and signals end of input when the pipe closes, so a command that reads until end of input finishes on its own:

```bash
    $ printf 'hello\n' | convox exec 7b6bccfd9fdf cat
    hello
```

Redirect a file the same way:

```bash
    $ convox exec 7b6bccfd9fdf "psql -1 -f -" < schema.sql
```

Pipes and redirections written to the right of `convox exec` are handled by your local shell. To use shell syntax inside the container, wrap the command in `sh -c`:

```bash
    $ convox exec 7b6bccfd9fdf "sh -c 'cat /etc/hosts | wc -l'"
```

Interactive sessions are unchanged.

### Error output

Output the command writes to standard error comes back with its standard output:

```bash
    $ convox exec 7b6bccfd9fdf "sh -c 'echo working; echo failed >&2'"
    working
    failed
```

Both streams arrive over one connection, so lines from the two can interleave. Do not depend on their order.

### Racks reached through the Console

On a Rack reached through the Console, or on a Convox Cloud machine, a command that reads standard input until end of input with nothing piped in never sees that end and waits until the session times out. Pipe at least one byte to release it, or start the command with [`convox run --detach --wait`](/reference/cli/run#detached-runs) instead.

## Exit Status

`convox exec` exits with the command's exit code. When the command's output stream ends without one, the CLI prints an error and exits `1`:

```text
the rack did not report an exit status for this command, so it may not have finished.
       Check the output above for a reason. The command may still be running in the target process, so retrying could run it twice.
       A command that must gate a deploy should write its own success marker to the output for the caller to check
```

Earlier CLI versions exited `0` here, so a CI step gated on `convox exec` passed while the command's outcome was unknown.

A stream lost in transit is caught by the CLI on its own and needs no Rack upgrade. An error the Rack raises before the command starts needs Rack 3.25.5 or later; earlier Racks send `0` as the exit status even when the command never ran.

## Version Requirements

- Basic `convox exec` functionality: All versions
- End of input signaled to a command reading piped input: Requires rack version >= 3.25.5. The change is on the rack, so it applies to any CLI version
- Error output returned from a command run without a terminal: Requires rack version >= 3.25.5. The change is on the rack, so it applies to any CLI version
- Failing on a missing exit status: The CLI catches a stream lost in transit on its own; an error the rack raises before the command starts requires rack version >= 3.25.5

## See Also

- [One-off Commands](/management/run) for running commands in containers
- [run](/reference/cli/run) for running a command in a new process
- [ps](/reference/cli/ps) for listing the processes you can exec into
