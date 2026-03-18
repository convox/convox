---
title: "test"
slug: test
url: /reference/cli/test
---
# test

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