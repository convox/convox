---
title: "exec"
description: "The convox exec command runs a command inside an existing running process, for example opening a shell in a container to inspect or debug it."
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

## See Also

- [One-off Commands](/management/run) for running commands in containers