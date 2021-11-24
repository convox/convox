---
title: "run"
draft: false
slug: run
url: /reference/cli/run
---
# run

## run

Execute a command in a new process

### Usage
```html
    convox run <service> <command>
```
### Examples
```html
    $ convox run web sh
    /usr/src/app #
```
Run against a specific release:
```html
    $ convox run --release RABCDEFGHIJ web sh
    /usr/src/app #
```