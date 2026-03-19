---
title: "apps"
slug: apps
url: /reference/cli/apps
---
# apps

## apps

List apps

### Usage
```bash
    convox apps
```
### Examples
```bash
    $ convox apps
    APP          STATUS   RELEASE
    myapp        running  RABCDEFGHI
    myapp2       running  RIHGFEDCBA
```
## apps cancel

Cancel an app update

### Usage
```bash
    convox apps cancel [app]
```
### Examples
```bash
    $ convox apps cancel
    Cancelling deployment of myapp... OK
```
## apps create

Create an app

### Usage
```bash
    convox apps create [app]
```
### Examples
```bash
    $ convox apps create myapp
    Creating myapp... OK
```
## apps delete

Delete an app

### Usage
```bash
    convox apps delete <app>
```
### Examples
```bash
    $ convox apps delete myapp
```
## apps export

Export an app

### Usage
```bash
    convox apps export [app]
```
### Flags

| Flag | Short | Description |
| ---- | ----- | ----------- |
| `--file` | `-f` | Export to file |

### Examples
```bash
    $ convox apps export --file myapp.tgz
    Exporting app myapp... OK
    Exporting env... OK
    Exporting build BABCDEFGHI... OK
    Exporting resource database... OK
    Packaging export... OK
```
## apps import

Import an app

### Usage
```bash
    convox apps import [app]
```
### Flags

| Flag | Short | Description |
| ---- | ----- | ----------- |
| `--file` | `-f` | Import from file |

### Examples
```bash
    $ convox apps import myapp2 --file myapp.tgz
    Creating app myapp2... OK
    Importing build... OK, RIHGFEDCBA
    Importing env... OK, RJIHGFEDCB
    Promoting RJIHGFEDCB... OK
    Importing resource database... OK
```
## apps info

Get information about an app

### Usage
```bash
    convox apps info [app]
```
### Examples
```bash
    $ convox apps info
    Name        myapp
    Status      running
    Generation  3
    Locked      false
    Release     RABCDEFGHI
```
## apps lock

Enable termination protection

### Usage
```bash
    convox apps lock [app]
```
### Examples
```bash
    $ convox apps lock
    Locking myapp... OK
```
## apps params

Display app parameters

### Usage
```bash
    convox apps params [app]
```
### Examples
```bash
    $ convox apps params -a myapp
    BuildCpu     0
    BuildLabels
    BuildMem     0
```

## apps params set

Set app parameters

### Usage
```bash
    convox apps params set <key=value> [key=value]... [app]
```
### Examples
```bash
    $ convox apps params set BuildCpu=1024 BuildMem=4096 -a myapp
    Setting parameters... OK
```

## apps unlock

Disable termination protection

### Usage
```bash
    convox apps unlock [app]
```
### Examples
```bash
    $ convox apps unlock
    Unlocking myapp... OK
```

## See Also

- [App](/reference/primitives/app) for app primitives
- [App Parameters](/configuration/app-parameters) for available app parameters
- [Deploy](/reference/cli/deploy) for deploying apps