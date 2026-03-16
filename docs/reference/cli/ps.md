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
    convox ps info
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
    convox ps stop
```
### Examples
```bash
    $ convox ps stop 62942430327e
    Stopping 62942430327e... OK
```