---
title: "cp"
slug: cp
url: /reference/cli/cp
---
# cp

The `convox cp` command copies files between your local filesystem and a running process. It is useful for debugging, inspecting container state, or extracting log files and artifacts from running containers. Use `convox ps` to find the process ID.

## cp

Copy files

### Usage
```bash
    convox cp <[pid:]src> <[pid:]dst>
```
### Examples
```bash
    $ convox cp 7b6bccfd9fdf:/root/test.sh .
```

The path format uses `<pid>:<path>` to reference files inside a running process. You can find process IDs with `convox ps`.

```bash
    $ convox cp ./local-file.txt web-0123456789-abcde:/tmp/file.txt
```