---
title: "Build Commands"
description: "The convox cloud build commands create, list, cancel, export, import, and inspect builds for an application and view their build logs."
slug: builds
url: /cloud/cli-reference/builds
---

# Build Commands

### build

Create a new build for an application.

```bash
$ convox cloud build [dir] -a <app> -i <machine>
```

**Options:**
- `--build-args`: Build-time arguments
- `--description`: Build description
- `--development`: Development build
- `--external`: Use external builder
- `--manifest`: Manifest file (default: convox.yml)
- `--no-cache`: Disable build cache

**Example:**
```bash
$ convox cloud build . -a myapp -i production --description "Feature update"
Packaging source... OK
Uploading source... OK
Starting build... OK
Build:   BABCDEFGHI
Release: RABCDEFGHI
```

### builds

List builds for an application.

```bash
$ convox cloud builds -a <app> -i <machine>
```

**Options:**
- `--limit`: Number of builds to show
- `--watch`: Watch for updates

**Example:**
```bash
$ convox cloud builds -a myapp -i production --limit 5
ID           STATUS    RELEASE      STARTED       ELAPSED  DESCRIPTION
BABCDEFGHI   complete  RABCDEFGHI   1 hour ago    2m       Feature update
BBCDEFGHIJ   complete  RBCDEFGHIJ   2 hours ago   3m
```

### builds cancel

Cancel a Build that is still running.

```bash
$ convox cloud builds cancel <build> -a <app> -i <machine>
```

**Example:**
```bash
$ convox cloud builds cancel BABCDEFGHI -a myapp -i production
Cancelling build BABCDEFGHI... OK
```

Cancelling deletes the Build's pod and sets the Build to `failed`; `convox cloud builds info` then shows a `Reason` of `cancelled by <actor>`. Only a Build the machine is running in a build pod can be cancelled, and any other Build returns `ERROR: build <build> is not running`. Requires machine version 3.25.7 or later. See [builds](/reference/cli/builds#builds-cancel) for the full reference.

### builds export

Export a build.

```bash
$ convox cloud builds export <build> -a <app> -i <machine>
```

**Options:**
- `--file`: Output file path

### builds import

Import a build.

```bash
$ convox cloud builds import -a <app> -i <machine>
```

**Options:**
- `--file`: Input file path

### builds info

Get information about a specific build.

```bash
$ convox cloud builds info <build> -a <app> -i <machine>
```

### builds logs

View logs for a build.

```bash
$ convox cloud builds logs <build> -a <app> -i <machine>
```

**Example:**
```bash
$ convox cloud builds logs BABCDEFGHI -a myapp -i production
Building: .
Step 1/5 : FROM node:22
...
Successfully built abc123def456
```
