---
title: "releases"
slug: releases
url: /reference/cli/releases
---
# releases

## releases

List releases for an app

### Usage
```bash
    convox releases
```
### Examples
```bash
    $ convox releases
    ID          STATUS  BUILD        CREATED         DESCRIPTION
    RIABCDEFGH          BJABCDEFGHI  30 seconds ago
    RABCDEFGHI  active  BABCDEFGHIJ  2 weeks ago
    RBCDEFGHIJ          BBCDEFGHIJK  2 weeks ago
```
## releases info

Get information about a release

### Usage
```bash
    convox releases info <release>
```
### Examples
```bash
    $ convox releases info RABCDEFGHI
    Id           RABCDEFGHI
    Build        BABCDEFGHIJ
    Created      2026-03-18T15:37:38Z
    Description
    Env
```
## releases create-from

Create a new release using a build from one release and env from another. This is useful for combining specific builds and environments, cross-app deployments, and build-once-deploy-many workflows.

### Usage
```bash
    convox releases create-from [options]
```

### Flags

| Flag | Description |
|------|-------------|
| `--build-from` | Release ID to use as the build source |
| `--env-from` | Release ID to use as the environment source |
| `--use-active-release-build` | Use the currently active release's build |
| `--use-active-release-env` | Use the currently active release's environment |
| `--promote` | Automatically promote the new release after creation |

### Examples

Create a new release using build from one release and environment from another:
```bash
    $ convox releases create-from --build-from=RXXXXXXXXXXX --env-from=RYYYYYYYYYY -a myapp
    Creating release... OK
    Release: RNEWRELEASE
```

Create and automatically promote the new release:
```bash
    $ convox releases create-from --build-from=RXXXXXXXXXXX --env-from=RYYYYYYYYYY -a myapp --promote
    Creating release... OK
    Release: RNEWRELEASE
    Promoting RNEWRELEASE... OK
```

Use the currently active release's build with environment from a specific release:
```bash
    $ convox releases create-from --use-active-release-build --env-from=RYYYYYYYYYY -a myapp
    Creating release... OK
    Release: RNEWRELEASE
```

Use the currently active release's environment with build from a specific release:
```bash
    $ convox releases create-from --build-from=RXXXXXXXXXXX --use-active-release-env -a myapp
    Creating release... OK
    Release: RNEWRELEASE
```

## releases manifest

Get the convox.yml manifest for a specific release.

### Usage
```bash
    convox releases manifest <release-id>
```
### Examples
```bash
    $ convox releases manifest RABCDEFGHIJ
    environment:
      - PORT=3000
    services:
      web:
        build: .
        port: 3000
```
## releases promote

Promote a release. If no release ID is specified, the most recent release is promoted.

### Usage
```bash
    convox releases promote [release-id]
```

### Flags

| Flag | Description |
|------|-------------|
| `--force` | Force promotion even if the release is already active |

### Examples
```bash
    $ convox releases promote RIABCDEFGH
    Promoting RIABCDEFGH...
    2026-03-18T20:55:37Z system/k8s/atom/app Status: Running => Pending
    2026-03-18T20:55:44Z system/k8s/web Scaled up replica set web-856bf5dbdf to 1
    2026-03-18T20:55:44Z system/k8s/web-856bf5dbdf-qkcm9 Successfully assigned convox-myapp/web-856bf5dbdf-qkcm9 to aks-default-22457946-vmss000000
    2026-03-18T20:55:44Z system/k8s/web-856bf5dbdf Created pod: web-856bf5dbdf-qkcm9
    2026-03-18T20:55:46Z system/k8s/web-856bf5dbdf-qkcm9 Pulling image "convoxctuntzfzqjho.azurecr.io/myapp:web.BJABCDEFGHI"
    2026-03-18T20:55:47Z system/k8s/web-856bf5dbdf-qkcm9 Successfully pulled image "convoxctuntzfzqjho.azurecr.io/myapp:web.BJABCDEFGHI"
    2026-03-18T20:55:48Z system/k8s/web-856bf5dbdf-qkcm9 Created container main
    2026-03-18T20:55:48Z system/k8s/web-856bf5dbdf-qkcm9 Started container main
    2026-03-18T20:55:54Z system/k8s/web Scaled down replica set web-7f58f4574 to 0
    2026-03-18T20:55:58Z system/k8s/atom/app Status: Pending => Updating
    2026-03-18T20:55:59Z system/k8s/atom/service/web Status: Running => Pending
    OK
```
## releases rollback

Copy an old release forward and promote it. This creates a new release with the same build and environment as the target release, then promotes it.

### Usage
```bash
    convox releases rollback <release-id>
```

### Flags

| Flag | Description |
|------|-------------|
| `--force` | Force the rollback even if the release is already active |

### Examples
```bash
    $ convox releases rollback RABCDEFGHI
    Rolling back to RABCDEFGHI... OK, RHIABCDEFG
    Promoting RHIABCDEFG...
    2026-03-18T20:58:01Z system/k8s/atom/app Status: Running => Pending
    2026-03-18T20:58:07Z system/k8s/web-95848bb45 Created pod: web-95848bb45-9fqts
    2026-03-18T20:58:07Z system/k8s/web-95848bb45-9fqts Successfully assigned convox-myapp/web-95848bb45-9fqts to aks-default-22457946-vmss000001
    2026-03-18T20:58:09Z system/k8s/web-95848bb45-9fqts Container image "convoxctuntzfzqjho.azurecr.io/myapp:web.BABCDEFGHIJ" already present on machine
    2026-03-18T20:58:09Z system/k8s/web-95848bb45-9fqts Created container main
    2026-03-18T20:58:10Z system/k8s/web-95848bb45-9fqts Started container main
    2026-03-18T20:58:14Z system/k8s/atom/app Status: Pending => Updating
    2026-03-18T20:58:20Z system/k8s/web-856bf5dbdf Deleted pod: web-856bf5dbdf-qkcm9
    2026-03-18T20:58:20Z system/k8s/web Scaled down replica set web-856bf5dbdf to 0
    2026-03-18T20:58:21Z system/k8s/atom/service/web Status: Running => Pending
    2026-03-18T20:58:33Z system/k8s/atom/service/web Status: Pending => Updating
    2026-03-18T20:58:33Z system/k8s/atom/app Status: Updating => Running
    2026-03-18T20:58:34Z system/k8s/atom/service/web Status: Updating => Running
    OK
```