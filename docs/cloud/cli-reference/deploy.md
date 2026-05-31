---
title: "Deployment Commands"
slug: deploy
url: /cloud/cli-reference/deploy
---

# Deployment Commands

### deploy

Build and promote in a single command.

```bash
$ convox cloud deploy [dir] -i <machine>
```

**Options:**
- `--app`: Target application (uses directory name if not specified)
- `--build-args`: Build-time arguments
- `--description`: Deployment description
- `--force`: Force deployment
- `--manifest`: Manifest file
- `--no-cache`: Disable build cache

**Example:**
```bash
$ convox cloud deploy . -i production --description "weekly release"
Packaging source... OK
Uploading source... OK
Starting build... OK
Build:   BABCDEFGHI
Release: RABCDEFGHI
Promoting RABCDEFGHI... OK
```
