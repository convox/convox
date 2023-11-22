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

### Flags

 - `--app`: String. Specifies the app name
 - `--cpu`: Number. Specifies the millicpu units of requests to set for the process.
 - `--cpu-limit cpu-limit`: Number. Specifies the millicpu units of limit to set for the process.
 - `--detach`: Boolean. To run in detach mode.
 - `--entrypoint`: String. Specifies the enntrypoint.
 - `--memory`: Number. Specifies the memory megabytes of requests to set for the process.
 - `--memory-limit`: Number. Specifies the memory megabytes of limit to set for the process.
 - `--rack`: String. Specifies the rack name.
 - `--release`: String. Specifies the release.

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
