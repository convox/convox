---
title: "ps"
draft: false
slug: ps
url: /reference/cli/ps
---
# ps

## ps

List app processes

### Usage
```html
    convox ps
```
### Examples
```html
    $ convox ps
    ID            SERVICE  STATUS   RELEASE      STARTED     COMMAND
    62942430327e  web      running  RCRLBREFPBX  1 week ago
```
## ps info

Get information about a process

### Usage
```html
    convox ps info
```
### Examples
```html
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
```html
    convox ps stop
```
### Examples
```html
    $ convox ps stop 62942430327e
    Stopping 62942430327e... OK
```