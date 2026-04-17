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

The `--external` flag runs the Docker build on your local machine (or CI runner) instead of uploading the source to the rack for in-cluster building. Use this when:

- Your source directory is large (e.g., model weights, large assets) and uploads are slow or time out
- You want local Docker layer caching for faster rebuilds
- You are building from a CI pipeline that already has the source checked out

#### How It Works

A standard `convox build` packages the source directory into a tarball, uploads it through the rack's load balancer, and builds the image in-cluster. With `--external`, the flow changes:

1. The CLI creates a Build record on the rack via a small API call
2. The rack returns a container registry URL with embedded push credentials (ECR on AWS, ACR on Azure, GCR on GCP)
3. The CLI uploads only the `convox.yml` manifest to the rack
4. Docker builds the image locally using your source directory
5. The CLI pushes the built image directly to the rack's container registry
6. A Release is created on the rack referencing the pushed image

The source tarball never passes through the load balancer, eliminating upload-size and idle-timeout constraints.

#### Requirements

- Docker must be installed and running on the machine executing the build
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

> `convox deploy --external` uses the same flow and additionally promotes the Release after it is created. See [deploy](/reference/cli/deploy#external-builds).

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
- [Deploying Changes](/deployment/deploying-changes#external-builds) for the deployment-workflow view of external builds
