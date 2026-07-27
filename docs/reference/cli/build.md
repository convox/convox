---
title: "build"
description: "The convox build command creates a build from a source directory, with flags for build args, no-cache, alternate manifests, and external local builds."
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
| `--force` | | bool | Reduce the environment drop guard message to a one-line notice, and bypass the guard when `CONVOX_ENV_DROP_GUARD=strict`. Requires CLI version 3.25.1+ |
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

### Environment Drop Guard

Starting with CLI version 3.25.1, `convox build`, `convox deploy`, and `convox test` run a preflight check before creating a build. New builds inherit their environment from the app's newest release, which can differ from the currently running release when environment changes are staged with `convox env set` or `convox env unset` without `--promote`. If the release the build is about to inherit from is missing environment variables that are set in the running release, the CLI prints a warning to stderr and the build continues. Set `CONVOX_ENV_DROP_GUARD=strict` to make the check blocking instead.

> The check was blocking by default in CLI version 3.25.1. CLI version 3.25.2 changed the default to a warning and added `CONVOX_ENV_DROP_GUARD=strict` to restore the blocking behavior.

#### Behavior

| Condition | `CONVOX_ENV_DROP_GUARD` | `--force` | Result |
|-----------|-------------------------|-----------|--------|
| Drop pending | Unset, or any value other than `strict` | No | Warning on stderr, exit 0, the build proceeds |
| Drop pending | `strict` | No | Error on stderr, exit 1, no build is created |
| Drop pending | Any value | Yes | One-line notice on stderr, exit 0, the build proceeds |
| No drop, or the release lookup fails | Any value | Any | No output, the build proceeds |

`CONVOX_ENV_DROP_GUARD` is compared exactly and case-sensitively against the literal string `strict`. Any other value, including `STRICT` and the empty string, selects the warning behavior.

#### Default Output

```
    WARNING: this build will drop env var(s) that are set in your running release RABC123: SECRET_KEY

    These vars are present in the running release (RABC123) but missing from the latest release (RDEF456), which is what a new build inherits from. This usually happens after `convox env set` or `convox env unset` without --promote.

    To keep them, set them again with --promote before deploying, for example:
        convox env set SECRET_KEY=... --promote

    If this drop is intentional, pass --force to reduce this to a one-line notice. To make this check blocking, set CONVOX_ENV_DROP_GUARD=strict.
```

#### Strict Mode

With `CONVOX_ENV_DROP_GUARD=strict` and no `--force`, the same information is reported as an error, the command exits 1, and no build is created. This output is unchanged from CLI version 3.25.1:

```
    ERROR: this build will drop env var(s) that are set in your running release RABC123: SECRET_KEY

    These vars are present in the running release (RABC123) but missing from the latest release (RDEF456), which is what a new build inherits from. This usually happens after `convox env set` or `convox env unset` without --promote.

    To keep them, set them again with --promote before deploying, for example:
        convox env set SECRET_KEY=... --promote

    If you meant to drop these vars, or you believe this is a false alarm, re-run with --force
```

Set `CONVOX_ENV_DROP_GUARD=strict` for interactive use and for production deploy gates. The default is the warning so that CI pipelines which intentionally drop variables are not broken by a CLI upgrade.

#### Additional Information

- The check only triggers when one or more variables set in the running release are absent from the newest release. Adding new variables or changing values never triggers it.
- If the drop is intentional, `--force` replaces the multi-paragraph message with a one-line notice naming the dropped variables, and bypasses the check entirely when `CONVOX_ENV_DROP_GUARD=strict`.
- All guard output goes to stderr, so `convox build --id` stdout parsing is unaffected.
- The check runs entirely in the CLI using read-only API calls, so it works with apps on any rack version. It does not cover `convox builds import` or `convox builds import-image`.
- Because the check lives only in the CLI, Builds started from the Console do not run it.

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
