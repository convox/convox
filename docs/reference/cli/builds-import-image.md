---
title: "builds import-image"
description: "The convox builds import-image command imports a prebuilt container image into a new Build, pulling it from a source registry and creating a Release."
slug: builds-import-image
url: /reference/cli/builds-import-image
---
# builds import-image

Import a prebuilt container image into a new Build. The Rack pulls the image from the source registry (using [skopeo](https://github.com/containers/skopeo)) and pushes it to the Rack's internal registry. A Release is created automatically on success.

This command requires a `convox.yml` manifest to define the App's Services. By default it reads `convox.yml` from the current directory.

### Usage
```bash
    convox builds import-image <source-image>
```

### Flags

| Flag | Short | Description |
| ---- | ----- | ----------- |
| `--rack` | `-r` | Rack name |
| `--app` | `-a` | App name |
| `--manifest` | `-m` | Path to convox.yml manifest (default: `convox.yml`) |
| `--src-creds-user` | | Source registry username |
| `--src-creds-pass` | | Source registry password (deprecated; use `--src-creds-pass-env` or `--src-creds-pass-stdin`) |
| `--src-creds-pass-env` | | Read source registry password from the named environment variable |
| `--src-creds-pass-stdin` | | Read source registry password from stdin (single line) |

Only one of `--src-creds-pass`, `--src-creds-pass-env`, or `--src-creds-pass-stdin` may be specified. The `--src-creds-pass` flag is deprecated and will be rejected in 3.25.0 because it exposes credentials in process listings.

### Examples

Import a public image:
```bash
    $ convox builds import-image registry.example.com/myapp:v1.2.3 -a myapp
    Creating build... OK, BABCDEFGHIJ
    Relaying image registry.example.com/myapp:v1.2.3... OK
    Waiting for import to complete... OK
    Creating release... OK, RABCDEFGHIJ
    Build:   BABCDEFGHIJ
    Release: RABCDEFGHIJ
```

Import from a private registry using an environment variable for credentials:
```bash
    $ export REGISTRY_PASS=my-secret-token
    $ convox builds import-image registry.example.com/myapp:v1.2.3 -a myapp \
        --src-creds-user myuser \
        --src-creds-pass-env REGISTRY_PASS
    Creating build... OK, BABCDEFGHIJ
    Relaying image registry.example.com/myapp:v1.2.3... OK
    Waiting for import to complete... OK
    Creating release... OK, RABCDEFGHIJ
    Build:   BABCDEFGHIJ
    Release: RABCDEFGHIJ
```

Import from a private registry using stdin for credentials:
```bash
    $ echo "$REGISTRY_PASS" | convox builds import-image registry.example.com/myapp:v1.2.3 -a myapp \
        --src-creds-user myuser \
        --src-creds-pass-stdin
```

Use a custom manifest path:
```bash
    $ convox builds import-image registry.example.com/myapp:v1.2.3 -a myapp -m deploy/convox.yml
```

## See Also

- [builds](/reference/cli/builds) for listing, exporting, and inspecting Builds
- [Build](/reference/primitives/app/build) for Build concepts and build arguments
- [deploy](/reference/cli/deploy) for building and promoting in one step
- [registries](/reference/cli/registries) for managing external registry credentials
