---
title: "Build"
draft: false
slug: Build
url: /reference/primitives/app/build
---
# Build

A Build is a compiled version of the code for each [Service](/reference/primitives/app/service) of an [App](/reference/primitives/app).

Convox uses `docker` to compile code into a Build.

## Definition

You can define the location to build for each [Service](/reference/primitives/app/service) in [`convox.yml`](/configuration/convox-yml).
```html
    services:
      api:
        build: ./api
      web:
        build: ./web
        manifest: Dockerfile.production
```
## Command Line Interface

### Creating a Build
```html
    $ convox build -a myapp
    Packaging source... OK
    Uploading source... OK
    Starting build... OK
    ...
    Build: BABCDEFGHI
    Release: RBCDEFGHIJ
```
> Every time a Build is created a new [Release](/reference/primitives/app/release) is created that references the Build.

### Listing Builds
```html
    $ convox builds -a myapp
    ID          STATUS    RELEASE     STARTED       ELAPSED
    BABCDEFGHI  complete  RBCDEFGHIJ  1 minute ago  25s
```
### Getting Information about a Build
```html
    $ convox builds info BABCDEFGHI -a myapp
    ID           BABCDEFGHI
    Status       complete
    Release      RBCDEFGHIJ
    Description
    Started      1 minute ago
    Elapsed      25s
```
### Getting logs for a Build
```html
    $ convox builds logs BABCDEFGHI -a myapp
    Sending build context to Docker daemon  4.2MB
    Step 1/34 : FROM golang AS development
    ...
```
### Exporting a Build
```html
    $ convox builds export BABCDEFGHI -a myapp -f /tmp/build.tgz
    Exporting build... OK
```
### Importing a Build
```html
    $ convox builds import -a myapp2 -f /tmp/build.tgz
```
> Importing a Build creates a new [Release](/reference/primitives/app/release) that references the Build.