---
title: "build"
slug: build
url: /reference/cli/build
---
# build

## build

Create a build

### Usage
```bash
    convox build [dir]
```

### Flags

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--build-args` | | string | Build arguments (repeatable). Requires rack version 3.22.0+ |
| `--description` | `-d` | string | Description for the build |
| `--development` | | bool | Build in development mode |
| `--external` | | bool | Use external build |
| `--id` | | bool | Output only the build ID |
| `--manifest` | `-m` | string | Path to an alternate manifest file |
| `--no-cache` | | bool | Build without using the Docker cache |
| `--wildcard-domain` | | bool | Use wildcard domain for the build |

### Examples
```bash
    $ convox build --no-cache --description "My latest build"
    Packaging source... OK
    Uploading source... OK
    Starting build... OK
    Authenticating https://index.docker.io/v1/: Login Succeeded
    Authenticating 1234567890.dkr.ecr.us-east-1.amazonaws.com: Login Succeeded
    Building: .
    ...
    ...
    Running: docker tag convox/myapp:web.BABCDEFGHI 1234567890.dkr.ecr.us-east-1.amazonaws.com/test-regis-1mjiluel3aiv3:web.BABCDEFGHI
    Running: docker push 1234567890.dkr.ecr.us-east-1.amazonaws.com/test-regis-1mjiluel3aiv3:web.BABCDEFGHI
    Build:   BABCDEFGHI
    Release: RABCDEFGHI
```

### External Builds

> External builds are available since Convox **3.0.50**.

The `--external` flag runs the Docker build **locally** on your machine (or CI runner) instead of uploading the source to the rack for in-cluster building. This is useful when:

- Your source directory is large (e.g., ML model weights, large assets)
- Source uploads are timing out through the load balancer
- You want to leverage local Docker layer caching for faster builds
- You are building from a CI pipeline that already has the source checked out

#### How It Works

A standard build packages the entire source directory into a tarball, uploads it through the rack's load balancer, and builds the image in-cluster. With `--external`, the flow changes:

1. The CLI creates a build record on the rack (a small API call)
2. The rack returns container registry credentials (ECR on AWS, ACR on Azure)
3. Docker builds the image **locally** using your source directory
4. The CLI pushes the built image **directly** to the rack's container registry
5. A release is created on the rack referencing the pushed image

The source tarball never passes through the load balancer, so there are no upload size or timeout constraints.

#### Requirements

- Docker must be running on the machine executing the build
- The machine must have network access to the rack's container registry

#### Example

```bash
    $ convox build --external -a myapp
    Building: .
    Sending build context to Docker daemon  2.51GB
    Step 1/10 : FROM python:3.11-slim AS base
     ---> a1b2c3d4e5f6
    ...
    Running: docker push 1234567890.dkr.ecr.us-east-1.amazonaws.com/test-regis-1mjiluel3aiv3:web.BABCDEFGHI
    Build:   BABCDEFGHI
    Release: RABCDEFGHI
```

> **Note:** `convox deploy --external` works the same way but also promotes the release after building.

### Pass build time env vars

You can pass env vars that will only exist at build time.

> Build arguments require rack version 3.22.0 or later.

```bash
    $ convox build --description "My Test Build" --build-args "BUILD_ENV1=val1" --build-args "BUILD_ENV2=val2"
    Packaging source... OK
    Uploading source... OK
    Starting build... OK
    Authenticating https://index.docker.io/v1/: Login Succeeded
    Authenticating 1234567890.dkr.ecr.us-east-1.amazonaws.com: Login Succeeded
    Building: .
    ...
    ...
    Running: docker tag convox/myapp:web.BABCDEFGHI 1234567890.dkr.ecr.us-east-1.amazonaws.com/test-regis-1mjiluel3aiv3:web.BABCDEFGHI
    Running: docker push 1234567890.dkr.ecr.us-east-1.amazonaws.com/test-regis-1mjiluel3aiv3:web.BABCDEFGHI
    Build:   BABCDEFGHI
    Release: RABCDEFGHI
```

## See Also

- [Build](/reference/primitives/app/build) for build concepts and build arguments
- [Deploy](/reference/cli/deploy) for building and promoting in one step
