---
title: "deploy"
slug: deploy
url: /reference/cli/deploy
---
# deploy

## deploy

Create and promote a build

### Usage
```bash
    convox deploy [dir]
```

### Flags

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--build-args` | | string | Build arguments (repeatable). Requires rack version 3.22.0+ |
| `--description` | `-d` | string | Description for the build |
| `--development` | | bool | Build in development mode |
| `--external` | | bool | Use external build |
| `--force` | | bool | Force deployment |
| `--id` | | bool | Output only the build/release ID |
| `--manifest` | `-m` | string | Path to an alternate manifest file |
| `--no-cache` | | bool | Build without using the Docker cache |
| `--wildcard-domain` | | bool | Use wildcard domain for the build |

### Examples
```bash
    $ convox deploy
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
    Promoting RABCDEFGHI...
    ...
    ...
    2026-03-18T15:41:16Z system/k8s/atom/app Status: Running => Pending
    2026-03-18T15:41:18Z system/k8s/atom/app Status: Pending => Updating
    2026-03-18T15:41:27Z system/k8s/atom/app Status: Updating => Running
    OK
```

### Pass build time env vars

You can pass env vars that will only exist at build time.

> Build arguments require rack version 3.22.0 or later.

```bash
    $ convox deploy --build-args "BUILD_ENV1=val1" --build-args "BUILD_ENV2=val2"
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
    Promoting RABCDEFGHI...
    ...
    ...
    2026-03-18T15:41:16Z system/k8s/atom/app Status: Running => Pending
    2026-03-18T15:41:18Z system/k8s/atom/app Status: Pending => Updating
    2026-03-18T15:41:27Z system/k8s/atom/app Status: Updating => Running
    OK
```

## See Also

- [Deploying Changes](/deployment/deploying-changes) for deployment workflow
- [Rolling Updates](/deployment/rolling-updates) for deployment strategies
- [deploy-debug](/reference/cli/deploy-debug) for diagnosing failed deployments
