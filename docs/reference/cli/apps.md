---
title: "apps"
description: "The convox apps command lists, creates, and deletes apps and manages app-specific operations such as locks, parameters, and import or export."
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
    Cancelling deployment of myapp...
    Rewriting last active release...
    OK
```

`convox apps cancel` cancels a deploy that is in progress and rolls the App back to the Release it was running before. On AWS that includes a deploy still waiting for a new RDS or ElastiCache resource to become available, on Rack version `3.25.10` or later; an earlier Rack returns `not currently updating` during that wait. It returns `app is not updating` when no deploy is running, including while a failed deploy is already rolling back. After the cancel, the CLI creates a new Release with the same build, environment and description as the App's most recent Release, without promoting it. It does not affect Builds. To cancel a Build the Rack is still running, use [`convox builds cancel`](/reference/cli/builds#builds-cancel).

A cancel does not remove an RDS or ElastiCache resource the cancelled deploy added. The instance keeps provisioning and is billed, `convox resources` does not list it, and the next promote whose `convox.yml` omits it deletes it, including the re-promote that `convox apps params set`, `convox apps lock` and `convox apps unlock` run. A resource the cancelled deploy removed from `convox.yml` is still deleted.

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
    Deleting myapp... OK
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
    BuildCpu     1000
    BuildMem     4096
```

Only parameters that are set are listed. A parameter you have not set does not appear.

## apps params set

Set app parameters

### Usage
```bash
    convox apps params set <Key=Value> [Key=Value]...
```
### Examples
```bash
    $ convox apps params set BuildCpu=1000 BuildMem=4096 -a myapp
    Updating parameters...
    ...
    OK
```

The App is selected with `-a`. Setting a parameter re-promotes the App's current Release, so the command streams that rollout and prints `OK` once the App is running again.

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