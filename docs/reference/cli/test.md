---
title: "test"
description: "The convox test command builds the app and runs the test command defined on each service in convox.yml, failing if any returns a non-zero exit code or reports no exit code at all."
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

## Exit Status

`convox test` fails when a service's test command returns a non-zero exit code. It also fails when a test command's output stream ends without an exit code at all:

```text
the rack did not report an exit status for this command, so it may not have finished.
       Check the output above for a reason. This test's process has been stopped.
       A command that must gate a deploy should write its own success marker to the output for the caller to check
```

Earlier CLI versions exited `0` here, so a pipeline gated on `convox test` passed while the result was unknown.

A stream lost in transit is caught by the CLI on its own and needs no Rack upgrade. An error the Rack raises before the test command starts needs Rack 3.25.5 or later; earlier Racks send `0` as the exit status even when the command never ran.

## Version Requirements

- Basic `convox test` functionality: All versions
- `--force`: Requires CLI version >= 3.25.1
- Failing on a missing exit status: The CLI catches a stream lost in transit on its own; an error the rack raises before the test command starts requires rack version >= 3.25.5

## See Also

- [Service](/reference/primitives/app/service) for configuring the `test` attribute
- [CI/CD Workflows](/deployment/workflows) for running tests in deployment pipelines
- [run](/reference/cli/run) for gating a pipeline step on a one-off command