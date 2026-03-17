---
title: "Running Locally"
slug: running-locally
url: /development/running-locally
---

# Running Locally

Running your application locally requires a [Development Rack](/installation/development-rack) to be installed.

## Starting Your Application

Navigate to the directory containing your application code and run:

```bash
convox start
```

This will:

1. Build your application using the `Dockerfile` in the current directory
2. Push the image to the local registry
3. Deploy your application to the local Rack
4. Sync local file changes into the running containers

```bash
$ convox start
build  | uploading source
build  | starting build
build  | Building: .
build  | ...
build  | Successfully built abc123
convox | starting sync from . to /app on web
web    | Listening on port 3000
```

### Code Sync

Convox automatically synchronizes your local changes up to the Development Rack so that you can work
using your favorite editor.

All files or directories that appear in a `COPY` or `ADD` directive in your `Dockerfile` will be
synchronized.

> Files or directories that appear in `.dockerignore` will not be synchronized.

### Options

| Flag | Description |
|------|-------------|
| `--manifest` | Specify an alternate manifest file (default: `convox.yml`) |
| `--no-build` | Skip the build step and use the existing image |
| `--no-cache` | Build without using the Docker cache |
| `--no-sync` | Disable file synchronization |

## Development Target

You can use a build target named `development` in your `Dockerfile` to work locally on an application that will be
later compiled to a binary and have its source code removed before deploying to production.

```dockerfile
FROM golang:1.22 AS development
ENV DEVELOPMENT=true
COPY . .
RUN go install ./cmd/web
CMD ["go", "run", "./cmd/web"]

FROM ubuntu:24.04 AS production
COPY --from=development /go/bin/web /app/web
CMD ["/app/web"]
```

During `convox start` only the `development` stage of the Dockerfile is used. This lets you run a version of your application that includes source code, compilers, and development tools.

When deploying to production with `convox deploy`, the entire Dockerfile runs and produces a minimal image with just the compiled binary.

## Accessing Your Application

Once your application is running, find its URL with:

```bash
convox services
```

```
SERVICE  DOMAIN                               PORTS
web      web.myapp.dev.localdev.convox.cloud  443:3000
```

Navigate to the domain shown in your browser. You will see a certificate warning because the local Rack uses self-signed certificates.

## See Also

- [Local Development Tutorial](/tutorials/local-development) for a guided walkthrough
- [Development Rack](/installation/development-rack) for setting up a local rack
