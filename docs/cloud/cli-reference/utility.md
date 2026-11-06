---
title: "Utility Commands"
description: "The convox cloud cp command copies files to and from processes, and convox cloud test runs an application's test suite on a machine."
slug: utility
url: /cloud/cli-reference/utility
---

# Utility Commands

### cp

Copy files to/from processes.

```bash
$ convox cloud cp <[pid:]src> <[pid:]dst> -a <app> -i <machine>
```

**Options:**
- `--tar-extra`: Extra tar options

**Examples:**
```bash
# Copy from container to local
$ convox cloud cp web-abc123:/app/config.json . -a myapp -i production

# Copy from local to container
$ convox cloud cp ./data.csv web-abc123:/tmp/ -a myapp -i production
```

### test

Run test suite for an application.

```bash
$ convox cloud test -a <app> -i <machine>
```

**Example:**
```bash
$ convox cloud test -a myapp -i staging
Running tests...
OK
```
