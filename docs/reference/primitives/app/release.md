---
title: "Release"
draft: false
slug: Release
url: /reference/primitives/app/release
---
# Release

A Release is the atomic unit of deployment consisting of a [Build](/reference/primitives/app/build) and a set of
[Environment Variables](/configuration/environment).

A Release is created every time you create a new [Build](/reference/primitives/app/build) or
change the [App](/reference/primitives/app)'s [Environment](/configuration/environment).

Promoting a Release will begin a [rolling deployment](/deployment/rolling-updates) that will continue
until the new Release is active on all [Processes](/reference/primitives/app/process) or
has been completely rolled back.

## Command Line Interface

### Creating a Release

#### On Build Creation
```html
    $ convox build -a myapp
    Packaging source... OK
    Uploading source... OK
    Starting build... OK
    ...
    Build: BABCDEFGHI
    Release: RBCDEFGHIJ
```
#### On Environment Change
```html
    $ convox env set FOO=bar -a myapp
    Setting FOO... OK
    Release: RCDEFGHIJK
```
### Listing Releases
```html
    $ convox releases -a myapp
    ID          STATUS  BUILD       CREATED        DESCRIPTION
    RCDEFGHIJK          BABCDEFGHI  1 minute ago   env add:FOO
    RBCDEFGHIJ  active  BABCDEFGHI  5 minutes ago  build 0a1b2c3d4e5f my commit message
```
### Getting Information about a Release
```html
    $ convox releases info RCDEFGHIJK -a myapp
    Id           RCDEFGHIJK
    Build        BABCDEFGHI
    Created      2019-01-01:00:00:00Z
    Description  env add:FOO
    Env          FOO=bar
```
### Promoting a Release
```html
    $ convox releases promote RCDEFGHIJK -a myapp
    Promoting RCDEFGHIJK...
    2019-01-01T00:00:49Z system/k8s/atom/app Status: Running => Pending
    2019-01-01T00:00:51Z system/k8s/web Scaled up replica set web-745f845dc to 1
    2019-01-01T00:00:51Z system/k8s/web-745f845dc Created pod: web-745f845dc-rzl2q
    2019-01-01T00:00:51Z system/k8s/web-745f845dc-rzl2q Successfully assigned convox-myapp/web-745f845dc-rzl2q to instance-0a1b2c3d4e5f
    2019-01-01T00:00:51Z system/k8s/web-745f845dc-rzl2q Pulling image "registry.host/convox/myapp:web.BABCDEFGHI"
    2019-01-01T00:00:53Z system/k8s/atom/app Status: Pending => Updating
    2019-01-01T00:00:55Z system/k8s/web-745f845dc-rzl2q Successfully pulled image "registry.host/convox/myapp:web.BABCDEFGHI"
    2019-01-01T00:00:56Z system/k8s/web-745f845dc-rzl2q Created container main
    2019-01-01T00:00:56Z system/k8s/web-745f845dc-rzl2q Started container main
    OK
```
### Getting the Manifest for a Release
```html
    $ convox releases manifest RCDEFGHIJK -a myapp
    services:
      web:
        build: .
        command: bin/web
        port: 5000
```
### Rolling Back to a Previous Release
```html
    $ convox releases rollback RBCDEFGHIJ -a myapp
    Rolling back to RBCDEFGHIJ... OK, RDEFGHIJKL
    2019-01-01T00:00:49Z system/k8s/atom/app Status: Running => Pending
    2019-01-01T00:00:51Z system/k8s/web Scaled up replica set web-32f41a279 to 1
    2019-01-01T00:00:51Z system/k8s/web-32f41a279 Created pod: web-32f41a279-rzl2q
    2019-01-01T00:00:51Z system/k8s/web-32f41a279-rzl2q Successfully assigned convox-myapp/web-32f41a279-rzl2q to instance-0a1b2c3d4e5f
    2019-01-01T00:00:51Z system/k8s/web-32f41a279-rzl2q Pulling image "registry.host/convox/myapp:web.BABCDEFGHI"
    2019-01-01T00:00:53Z system/k8s/atom/app Status: Pending => Updating
    2019-01-01T00:00:55Z system/k8s/web-32f41a279-rzl2q Successfully pulled image "registry.host/convox/myapp:web.BABCDEFGHI"
    2019-01-01T00:00:56Z system/k8s/web-32f41a279-rzl2q Created container main
    2019-01-01T00:00:56Z system/k8s/web-745f845dc-rzl2q Started container main
    OK
```
> Rolling back to a previous Release makes a copy of the that Release and promotes the copy.