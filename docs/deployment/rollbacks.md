---
title: "Rollbacks"
draft: false
slug: Rollbacks
url: /deployment/rollbacks
---
# Rollbacks

You can easily roll back to an old [Release](/reference/primitives/app/release)

## Find the Release

First you will need to find the [Release](/reference/primitives/app/release) to which
you would like to roll back.
```html
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
```html
    $ convox releases rollback RBCDEFGHIJ -a myapp
    Rolling back to RBCDEFGHIJ...
    2019-01-01T00:00:49Z system/k8s/atom/app Status: Running => Pending
    2019-01-01T00:00:51Z system/k8s/web Scaled up replica set web-745f845dc to 1
    2019-01-01T00:00:51Z system/k8s/web-745f845dc Created pod: web-745f845dc-rzl2q
    2019-01-01T00:00:51Z system/k8s/web-745f845dc-rzl2q Successfully assigned convox-myapp/web-745f845dc-rzl2q to instance-0a1b2c3d4e5f
    2019-01-01T00:00:51Z system/k8s/web-745f845dc-rzl2q Pulling image "registry.host/convox/myapp:web.BABCDEFGHI"
    2019-01-01T00:00:53Z system/k8s/atom/app Status: Pending => Updating
    2019-01-01T00:00:55Z system/k8s/web-745f845dc-rzl2q Successfully pulled image "registry.host/convox/myapp:web.BABCDEFGHI"
    2019-01-01T00:00:56Z system/k8s/web-745f845dc-rzl2q Created container main
    2019-01-01T00:00:56Z system/k8s/web-745f845dc-rzl2q Started container main
    OK, RZYXWVUTSR
```

```html
$ convox releases -a myapp
ID          STATUS  BUILD       CREATED         DESCRIPTION
RZYXWVUTSR  active  BABCDEFGHI  1 minute ago    build 0a1b2c3d4e5f my commit message
RCDEFGHIJK          BABCDEFGHI  5 minutes ago   env add:FOO
RBCDEFGHIJ          BABCDEFGHI  10 minutes ago  build 0a1b2c3d4e5f my commit message
```