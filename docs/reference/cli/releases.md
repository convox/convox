---
title: "releases"
description: "The convox releases command lists an app's releases and manages them with info, promote, rollback, manifest, and create-from for build-once deploy-many flows."
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

Get information about a release.

### Usage
```bash
    convox releases info <release> [--reveal]
```

### Flags

| Flag | Type | Description |
|------|------|-------------|
| `--reveal` | bool | Show unmasked values for env keys in the mask list. See [env mask](/reference/cli/env#env-mask) |

### Examples
```bash
    $ convox releases info RABCDEFGHI
    Id           RABCDEFGHI
    Build        BABCDEFGHIJ
    Created      2026-03-18T15:37:38Z
    Description
    Env
```

The `Env` field respects the per-app mask list managed by `convox env mask`. On a TTY, values for masked keys render as `****`. Piped output and `--reveal` both show real values.

## releases create-from

Create a new release using a build from one release and env from another. This is useful for pairing a known-good build with a newer environment, or the reverse. Both source Releases must belong to the App the command targets.

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

Pass exactly one of `--build-from` and `--use-active-release-build`, and exactly one of `--env-from` and `--use-active-release-env`.

### Examples

Create a new release using build from one release and environment from another:
```bash
    $ convox releases create-from --build-from=RXXXXXXXXXXX --env-from=RYYYYYYYYYY -a myapp
    Using build from release: RXXXXXXXXXXX
    Using env from release: RYYYYYYYYYY
    Created release: RNEWRELEASE
    OK
```

Create and automatically promote the new release. The promote streams the rollout the same way [`convox releases promote`](#releases-promote) does:
```bash
    $ convox releases create-from --build-from=RXXXXXXXXXXX --env-from=RYYYYYYYYYY -a myapp --promote
    Using build from release: RXXXXXXXXXXX
    Using env from release: RYYYYYYYYYY
    Created release: RNEWRELEASE
    Promoting RNEWRELEASE...
    ...
    OK
```

Use the currently active release's build with environment from a specific release:
```bash
    $ convox releases create-from --use-active-release-build --env-from=RYYYYYYYYYY -a myapp
    Using build from release: RABCDEFGHIJ
    Using env from release: RYYYYYYYYYY
    Created release: RNEWRELEASE
    OK
```

Use the currently active release's environment with build from a specific release:
```bash
    $ convox releases create-from --build-from=RXXXXXXXXXXX --use-active-release-env -a myapp
    Using build from release: RXXXXXXXXXXX
    Using env from release: RABCDEFGHIJ
    Created release: RNEWRELEASE
    OK
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
| `--force` | Promote without waiting for an in-flight rollout of the App to finish. The Rack then accepts the promote while the App is updating |

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

Without `--force` the command waits for an in-flight rollout of the App to finish before it promotes. The Rack refuses a Release created before the active one with `can not promote an older release, try rollback`; use [`convox releases rollback`](#releases-rollback) for that. See [deploy: Failure Messages](/reference/cli/deploy#failure-messages) for the messages a promote prints when its rollout fails, when another promote replaces its Release, and when the rollout it waited behind fails.

## releases rollback

Copy an old release forward and promote it. This creates a new release with the same build and environment as the target release, then promotes it.

### Usage
```bash
    convox releases rollback <release-id>
```

### Flags

| Flag | Description |
|------|-------------|
| `--force` | Promote the new Release while the App is updating. Without it, a rollback during a rollout fails with `app is currently updating` |
| `--id` | Print only the new Release ID on stdout; progress goes to stderr |

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

See [deploy: Failure Messages](/reference/cli/deploy#failure-messages) for the messages a rollback prints when its rollout fails or another promote replaces its Release.

## See Also

- [Release](/reference/primitives/app/release) for release concepts
- [Rollbacks](/deployment/rollbacks) for rollback workflow
- [deploy-debug](/reference/cli/deploy-debug) for diagnosing why a promotion failed