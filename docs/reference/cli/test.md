---
title: "test"
description: "The convox test command builds the app and runs the test command defined on each service in convox.yml, failing if any returns a non-zero exit code."
slug: test
url: /reference/cli/test
---
# test

The `convox test` command builds the app and then runs the `test` command defined on each service in `convox.yml`. If any test command returns a non-zero exit code, the overall test fails.

## test

Run tests

### Usage
```bash
    convox test
```

### Flags

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--description` | `-d` | string | Description for the build |
| `--force` | | bool | Reduce the environment drop guard message to a one-line notice, and bypass the guard when `CONVOX_ENV_DROP_GUARD=strict`. Requires CLI version 3.25.1+ |
| `--release` | | string | Use an existing release to run tests instead of building |
| `--timeout` | `-t` | number | Timeout for the test run |

### Examples
```bash
    $ convox test
    Packaging source... OK
    Uploading source... OK
    Starting build... OK
    ...<Docker output>
```

Tests are defined using the `test` attribute on each service in `convox.yml`. See the [Service](/reference/primitives/app/service) reference for configuration details.

Because `convox test` creates a build, it runs the same [environment drop guard](/reference/cli/build#environment-drop-guard) as `convox build`: a warning is printed to stderr and the test run continues, unless `CONVOX_ENV_DROP_GUARD=strict` is set, in which case the run is blocked. `--force` reduces the message to a one-line notice and bypasses the guard in strict mode. When `--release` is passed, no build is created and the guard does not run.

## See Also

- [Service](/reference/primitives/app/service) for configuring the `test` attribute
- [CI/CD Workflows](/deployment/workflows) for running tests in deployment pipelines