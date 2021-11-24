---
title: "Running Locally"
draft: false
slug: Running Locally
url: /development/running-locally
---

# Running Locally

Running your application locally requires a [Development Rack](/installation/development-rack) to be installed.

## Starting Your Application

Once you have a Development Rack, you can go to the directory containing your application code and run `convox start`.
```html
    $ convox start
    build  | uploading source
    build  | starting build
    build  | Authenticating registry.dev.convox/convox: Login Succeeded
    build  | Building: .
    build  | Sending build context to Docker daemon  114.4MB
    build  | Step 1/5 : FROM golang:1.13 AS development
    build  |  ---> 272e3f68338f
    build  | Step 2/5 : ENV DEVELOPMENT=true
    build  |  ---> 8323381038aa
    build  | Step 3/5 : COPY . .
    build  |  ---> e87c93ad5c25
    build  | Step 4/5 : RUN go install ./cmd/web
    build  |  ---> 0be9da9a42c6
    build  | Step 5/5 : CMD ["bin/development", "web"]
    build  |  ---> e87c93ad5c25
    build  | Successfully built e87c93ad5c25
    build  | Successfully tagged 66608a93037391937ae7bdd4e148189d1369d38e:latest
    build  | Running: docker tag 66608a93037391937ae7bdd4e148189d1369d38e dev/myapp:web.BABCDEFGHI
    build  | Running: docker tag dev/myapp:web.BABCDEFGHI registry.dev.convox/myapp:web.BABCDEFGHI
    build  | Running: docker push registry.dev.convox/myapp:web.BABCDEFGHI
    convox | starting sync from . to . on web
    web    | Scaled up replica set web-786b6d8f5d to 1
    web    | Created pod: web-786b6d8f5d-l9jd2
    web    | Successfully assigned dev-convox/web-786b6d8f5d-l9jd2 to docker-desktop
    web    | Container image "registry.dev.convox/myapp:web.BABCDEFGHI" already present on machine
    web    | Created container main
    web    | make: '/go/bin/web' is up to date.
    web    | ns=web at=listen hostname="web.convox" proto="https" addr=":3000"
```
### Code Sync

Convox automatically synchronizes your local changes up to the Development Rack so that you can work
using your favorite editor.

All files or directories that appear in a `COPY` or `ADD` directive in your `Dockerfile` will be
synchronized.

> Files or directories that appear in `.dockerignore` will not be synchronized.

## Development Target

You can use a build target named `development` in your `Dockerfile` to work locally on an application that will be
later compiled to a binary and have its source code removed before deploying to production.
```html
    FROM golang:1.13 AS development
    ENV DEVELOPMENT=true
    COPY . .
    RUN go install ./cmd/web
    CMD ["bin/development", "web"]

    FROM ubuntu:18.04 as production
    ENV DEVELOPMENT=false
    COPY --from=development /go/bin/web /app/web
    CMD ["/app/web"]
```
In this example during `convox start` only the first section of the Dockerfile will be run. This allows you to
work in a container that contains the full source code where you can set `CMD` to a script that automatically
recompiles and reloads the application when the source code is changed.

When deploying to production the entire Dockerfile would be run and you would end up with a bare `ubuntu:18.04` container
with only the compiled binary copied into it.