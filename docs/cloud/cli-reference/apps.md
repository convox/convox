---
title: "Application Commands"
description: "The convox cloud apps commands list, create, delete, export, import, and inspect applications on a machine, plus view and set app parameters."
slug: apps
url: /cloud/cli-reference/apps
---

# Application Commands

### apps

List applications on a machine.

```bash
$ convox cloud apps -i <machine>
APP       STATUS   RELEASE
web-app   running  RABCDEFGHI
api       running  RBCDEFGHIJ
```

### apps cancel

Cancel an in-progress app update.

```bash
$ convox cloud apps cancel [app] -i <machine>
```

**Example:**
```bash
$ convox cloud apps cancel -a myapp -i production
Cancelling deployment of myapp... OK
```

### apps create

Create a new application on a machine.

```bash
$ convox cloud apps create [name] -i <machine>
```

**Options:**
- `--generation`: App generation (defaults to 3)
- `--timeout`: Creation timeout

**Example:**
```bash
$ convox cloud apps create myapp -i production
Creating myapp... OK
```

### apps delete

Delete an application from a machine.

```bash
$ convox cloud apps delete <app> -i <machine>
```

**Example:**
```bash
$ convox cloud apps delete oldapp -i production
Deleting oldapp... OK
```

### apps export

Export an application configuration and data.

```bash
$ convox cloud apps export [app] -i <machine>
```

**Options:**
- `--file`: Output file path

**Example:**
```bash
$ convox cloud apps export -a myapp -i production --file myapp-backup.tgz
Exporting app myapp... OK
```

### apps import

Import an application from an export file.

```bash
$ convox cloud apps import [app] -i <machine>
```

**Options:**
- `--file`: Input file path

**Example:**
```bash
$ convox cloud apps import -a myapp -i staging --file myapp-backup.tgz
Importing app myapp... OK
```

### apps info

Get detailed information about an application.

```bash
$ convox cloud apps info [app] -i <machine>
```

**Example:**
```bash
$ convox cloud apps info -a myapp -i production
Name        myapp
Status      running
Generation  3
Locked      false
Release     RABCDEFGHI
```

### apps params

View application parameters.

```bash
$ convox cloud apps params [app] -i <machine>
```

### apps params set

Set application parameters.

```bash
$ convox cloud apps params set <Key=Value> [Key=Value]... -a <app> -i <machine>
```

**Example:**
```bash
$ convox cloud apps params set BuildMem=2048 -a myapp -i production
Setting parameters... OK
```
