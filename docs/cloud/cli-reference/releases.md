---
title: "Release Management"
slug: releases
url: /cloud/cli-reference/releases
---

# Release Management

### releases

List releases for an application.

```bash
$ convox cloud releases -a <app> -i <machine>
```

**Options:**
- `--limit`: Number of releases to show
- `--watch`: Watch for updates

**Example:**
```bash
$ convox cloud releases -a myapp -i production
ID           STATUS  BUILD        CREATED        DESCRIPTION
RCDEFGHIJK           BABCDEFGHI   1 minute ago   env add:API_KEY
RABCDEFGHI   active  BABCDEFGHI   5 minutes ago  weekly release
```

### releases create-from

Create a release from existing builds and environments.

```bash
$ convox cloud releases create-from -a <app> -i <machine>
```

**Options:**
- `--build-from`: Source build
- `--env-from`: Source environment
- `--promote`: Auto-promote
- `--use-active-release-build`: Use active release build
- `--use-active-release-env`: Use active release environment

### releases info

Get information about a release.

```bash
$ convox cloud releases info <release> -a <app> -i <machine>
```

### releases manifest

View the manifest for a release.

```bash
$ convox cloud releases manifest <release> -a <app> -i <machine>
```

### releases promote

Promote a release to active.

```bash
$ convox cloud releases promote <release> -a <app> -i <machine>
```

**Options:**
- `--force`: Force promotion

**Example:**
```bash
$ convox cloud releases promote RCDEFGHIJK -a myapp -i production
Promoting RCDEFGHIJK... OK
```

### releases rollback

Rollback to a previous release.

```bash
$ convox cloud releases rollback <release> -a <app> -i <machine>
```

**Options:**
- `--force`: Force rollback

**Example:**
```bash
$ convox cloud releases rollback RABCDEFGHI -a myapp -i production
Rolling back to RABCDEFGHI... OK
```
