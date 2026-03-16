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
