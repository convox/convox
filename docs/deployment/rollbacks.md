---
title: "Rollbacks"
slug: rollbacks
url: /deployment/rollbacks
---
# Rollbacks

You can roll back to an old [Release](/reference/primitives/app/release)

## Find the Release

First you will need to find the [Release](/reference/primitives/app/release) to which
you would like to roll back.
```bash
    $ convox releases -a myapp
    ID          STATUS  BUILD       CREATED        DESCRIPTION
    RCDEFGHIJK  active  BABCDEFGHI  1 minute ago   env add:FOO
    RBCDEFGHIJ          BABCDEFGHI  5 minutes ago  build 0a1b2c3d4e5f my commit message
```
In this example we will assume that `RCDEFGHIJK` has caused a problem and we would like to
roll back to `RBCDEFGHIJ`

## Trigger the Rollback

Rolling back to an old [Release](/reference/primitives/app/release) will make a copy
of that [Release](/reference/primitives/app/release) and promote the copy.
```bash
    $ convox releases rollback RBCDEFGHIJ -a myapp
    Rolling back to RBCDEFGHIJ...
    2026-01-15T14:30:49Z system/k8s/atom/app Status: Running => Pending
    2026-01-15T14:30:51Z system/k8s/web Scaled up replica set web-745f845dc to 1
    2026-01-15T14:30:51Z system/k8s/web-745f845dc Created pod: web-745f845dc-rzl2q
    2026-01-15T14:30:51Z system/k8s/web-745f845dc-rzl2q Successfully assigned convox-myapp/web-745f845dc-rzl2q to instance-0a1b2c3d4e5f
    2026-01-15T14:30:51Z system/k8s/web-745f845dc-rzl2q Pulling image "registry.host/convox/myapp:web.BABCDEFGHI"
    2026-01-15T14:30:53Z system/k8s/atom/app Status: Pending => Updating
    2026-01-15T14:30:55Z system/k8s/web-745f845dc-rzl2q Successfully pulled image "registry.host/convox/myapp:web.BABCDEFGHI"
    2026-01-15T14:30:56Z system/k8s/web-745f845dc-rzl2q Created container main
    2026-01-15T14:30:56Z system/k8s/web-745f845dc-rzl2q Started container main
    OK, RZYXWVUTSR
```

```bash
$ convox releases -a myapp
ID          STATUS  BUILD       CREATED         DESCRIPTION
RZYXWVUTSR  active  BABCDEFGHI  1 minute ago    build 0a1b2c3d4e5f my commit message
RCDEFGHIJK          BABCDEFGHI  5 minutes ago   env add:FOO
RBCDEFGHIJ          BABCDEFGHI  10 minutes ago  build 0a1b2c3d4e5f my commit message
```

## How Rollbacks Work

Rolling back creates a new Release with the same build and environment as the target release. The original release is not modified. Environment variables from the target release are preserved in the new release.

Database migrations are not automatically reverted during a rollback. If you have applied destructive database changes since the target release, you may need to handle the migration rollback separately.

## See Also

- [Deploying Changes](/deployment/deploying-changes) for the standard deployment process
- [Rolling Updates](/deployment/rolling-updates) for understanding zero-downtime deployments
