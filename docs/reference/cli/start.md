---
title: "start"
description: "The convox start command runs an application locally for development, building services and syncing local code changes into the running containers."
slug: start
url: /reference/cli/start
---
# start

## start

Start an application for local development

### Usage
```bash
    convox start [service] [service...] [options]
```
### Flags

| Flag | Short | Description |
| ---- | ----- | ----------- |
| `--manifest` | `-m` | Manifest file (default: `convox.yml`) |
| `--external` | `-e` | Build the image with Docker on this machine and push it to the Rack. On GCP, requires Rack version 3.25.12 or later |
| `--generation` | `-g` | Generation |
| `--no-build` | | Skip building |
| `--no-cache` | | Build without the layer cache. A Build on the Rack requires CLI 3.25.12 or later and Rack version 3.25.11 or later; earlier versions build with the cache. With `--external` it applies to the local build on any version |
| `--no-sync` | | Do not sync local changes into running containers |
### Examples
```bash
    $ convox start
    build  | uploading source
    build  | starting build
    build  | #1 [internal] load build definition from Dockerfile
    build  | #1 transferring dockerfile: 186B done
    build  | #1 DONE 0.0s
    build  | ...
    build  | #8 [2/3] WORKDIR /usr/src/app
    build  | #8 CACHED
    build  | ...
    build  | #10 exporting to image
    build  | ...
    convox | starting sync from . to /usr/src/app on web
    web    | Scaled up replica set web-58d8446884 to 1
    web    | Created pod: web-58d8446884-gfkxz
    web    | Successfully assigned convox-nodejs/web-58d8446884-gfkxz to docker-desktop
    web    | Container image "registry.convox/nodejs:web.BABCDEFGHI" already present on machine
    web    | Created container main
    web    | Server running at http://0.0.0.0:3000/
    web    | Started container main
```

## See Also

- [Local Development](/tutorials/local-development) for a guided local development walkthrough
- [Running Locally](/development/running-locally) for code sync and build target details