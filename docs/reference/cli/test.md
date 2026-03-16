---
title: "test"
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
### Examples
```bash
    $ convox test
    Packaging source... OK
    Uploading source... OK
    Starting build... OK
    ...<Docker output>
```

Tests are defined using the `test` attribute on each service in `convox.yml`. See the [Service](/reference/primitives/app/service) reference for configuration details.

## See Also

- [Service](/reference/primitives/app/service) for configuring the `test` attribute
- [CI/CD Workflows](/deployment/workflows) for running tests in deployment pipelines