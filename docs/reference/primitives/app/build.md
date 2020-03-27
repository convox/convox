# Build

A Build is a compiled version of the code for each [Service](service.md) of an [App](../app).

Convox uses `docker` to compile code into a Build.

## Definition

You can define the location to build for each [Service](service.md) in [`convox.yml`](../convox-yml.md).

    services:
      api:
        build: ./api
      web:
        build: ./web
        manifest: Dockerfile.production

## Command Line Interface

### Creating a Build

    $ convox build -a myapp
    Packaging source... OK
    Uploading source... OK
    Starting build... OK
    ...
    Build: BABCDEFGHI
    Release: RBCDEFGHIJ

> Every time a Build is created a new [Release](release.md) is created that references the Build.

### Listing Builds

    $ convox builds -a myapp
    ID          STATUS    RELEASE     STARTED       ELAPSED
    BABCDEFGHI  complete  RBCDEFGHIJ  1 minute ago  25s

### Getting Information about a Build

    $ convox builds info BABCDEFGHI -a myapp
    ID           BABCDEFGHI
    Status       complete
    Release      RBCDEFGHIJ
    Description
    Started      1 minute ago
    Elapsed      25s

### Getting logs for a Build

    $ convox builds logs BABCDEFGHI -a myapp
    Sending build context to Docker daemon  4.2MB
    Step 1/34 : FROM golang AS development
    ...

### Exporting a Build

    $ convox builds export BABCDEFGHI -a myapp -f /tmp/build.tgz
    Exporting build... OK

### Importing a Build

    $ convox builds import -a myapp2 -f /tmp/build.tgz

> Importing a Build creates a new [Release](release.md) that references the Build.