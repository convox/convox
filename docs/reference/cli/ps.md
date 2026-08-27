---
title: "ps"
description: "The convox ps command lists an app's running processes and manages per-process operations such as info and stop, including exit status for a finished process and budget-cap sub-states."
slug: ps
url: /reference/cli/ps
---
# ps

## ps

List app processes

### Usage
```bash
    convox ps
```
### Flags

| Flag | Short | Description |
| ---- | ----- | ----------- |
| `--release` | | Filter by release |
| `--service` | `-s` | Filter by service |

### Examples
```bash
    $ convox ps
    ID            SERVICE  STATUS   RELEASE      STARTED     COMMAND
    62942430327e  web      running  RCRLBREFPBX  1 week ago
```

When the app's budget cap has been breached (3.24.6+), `convox ps` adds a
`BUDGET` column showing the per-process sub-state. Possible values:

| Value | Meaning |
|:------|:--------|
| `armed-Nm` | Auto-shutdown is armed; `N` minutes remain in the notify-before window. |
| `at-cap-keda` | Process belongs to a KEDA-managed Service that has been paused via `autoscaling.keda.sh/paused-replicas`. |
| `at-cap-auto` | Process belongs to a deployment-only Service that has been scaled to 0 by auto-shutdown. |
| `at-cap` | Process belongs to a Service whose deploys have been blocked (cap action `block-new-deploys`); existing replicas continue to run. |

The column is omitted when the cap is not tripped to keep healthy-state
output table-width-stable. See [Budget Caps](/management/budget-caps) for
the full sub-state lifecycle and recovery flow.

A one-off process started with [`convox run --detach --retain`](/reference/cli/run#detached-runs) stays in this listing after its command exits, with a status of `complete` or `failed`, for the length of its retention window. Retention requires rack version 3.25.5 or later; without it, a finished one-off process leaves the listing a few seconds after its command exits.

## ps info

Get information about a process

### Usage
```bash
    convox ps info <pid>
```
### Examples
```bash
    $ convox ps info 62942430327e
    Id        62942430327e
    App       nodejs
    Command
    Instance  i-0cbaa6d2dd1d094c0
    Release   RCRLBREFPBX
    Service   web
    Started   1 week ago
    Status    running
```

A finished process adds an `Exit` row carrying the primary container's exit status:

```bash
    $ convox ps info web-s43xf
    Id        web-s43xf
    App       nodejs
    Command   bin/migrate
    Instance  i-0cbaa6d2dd1d094c0
    Release   RCRLBREFPBX
    Service   web
    Started   2 minutes ago
    Status    complete
    Exit      0
```

The `Exit` row is absent while the process is still running, absent when no container ever started (the process was stopped before its command ran, or it never scheduled), and absent on racks before 3.25.5.

A one-off process is removed a few seconds after it finishes, so pass `--retain` or `--wait` to [`convox run`](/reference/cli/run#detached-runs) to keep its record readable long enough to inspect.

## ps stop

Stop a process

### Usage
```bash
    convox ps stop <pid>
```
### Examples
```bash
    $ convox ps stop 62942430327e
    Stopping 62942430327e... OK
```

## See Also

- [exec](/reference/cli/exec) for running commands in existing processes
- [run](/reference/cli/run) for running commands in new processes
- [scale](/reference/cli/scale) for adjusting process counts and resources
- [Detached Runs](/reference/cli/run#detached-runs) for keeping a finished one-off process readable