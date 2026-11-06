---
title: "Environment Commands"
description: "The convox cloud env commands list, edit, get, set, and unset environment variables for an application, with optional auto-promote."
slug: env
url: /cloud/cli-reference/env
---

# Environment Commands

### env

List environment variables for an application.

```bash
$ convox cloud env -a <app> -i <machine>
```

**Options:**
- `--release`: Specific release
- `--watch`: Watch for updates

**Example:**
```bash
$ convox cloud env -a myapp -i production
DATABASE_URL=postgres://localhost/myapp
NODE_ENV=production
```

### env edit

Edit environment variables interactively.

```bash
$ convox cloud env edit -a <app> -i <machine>
```

**Options:**
- `--promote`: Auto-promote after editing

### env get

Get a specific environment variable.

```bash
$ convox cloud env get <var> -a <app> -i <machine>
```

**Example:**
```bash
$ convox cloud env get DATABASE_URL -a myapp -i production
postgres://localhost/myapp
```

### env set

Set environment variables.

```bash
$ convox cloud env set <key=value> [key=value]... -a <app> -i <machine>
```

**Options:**
- `--promote`: Auto-promote after setting
- `--replace`: Replace all environment variables

**Example:**
```bash
$ convox cloud env set NODE_ENV=production API_KEY=secret -a myapp -i production
Setting NODE_ENV, API_KEY... OK
Release: RCDEFGHIJK
```

### env unset

Remove environment variables.

```bash
$ convox cloud env unset <key> [key]... -a <app> -i <machine>
```

**Options:**
- `--promote`: Auto-promote after unsetting

**Example:**
```bash
$ convox cloud env unset DEBUG_MODE -a myapp -i production
Unsetting DEBUG_MODE... OK
Release: RDEFGHIJKL
```
