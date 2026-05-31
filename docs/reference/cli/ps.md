---
title: "ps"
description: "The convox ps command lists an app's running processes and manages per-process operations such as info and stop, including budget-cap sub-states."
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