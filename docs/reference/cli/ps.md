---
title: "ps"
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