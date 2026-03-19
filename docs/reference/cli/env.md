---
title: "env"
slug: env
url: /reference/cli/env
---
# env

## env

List env vars

### Usage
```bash
    convox env [--release <id>]
```

### Flags

| Flag | Type | Description |
|------|------|-------------|
| `--release` | string | Show environment from a specific release |

### Examples
```bash
    $ convox env
    COUNT=0
    FOO=bar
    TEST=dummy

    $ convox env --release RABCDEFGHI
    COUNT=0
    FOO=bar
```

## env edit

Edit env interactively

### Usage
```bash
    convox env edit [--release <id>] [--promote]
```

### Flags

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--promote` | `-p` | bool | Promote the release after editing |
| `--release` | | string | Edit environment from a specific release |

### Examples
```bash
    $ convox env edit
    Setting ... OK
    Release: RABCDEFGHI
```

## env get

Get an env var

### Usage
```bash
    convox env get <var> [--release <id>]
```

### Flags

| Flag | Type | Description |
|------|------|-------------|
| `--release` | string | Get variable from a specific release |

### Examples
```bash
    $ convox env get FOO
    bar
```

## env set

Set env var(s)

### Usage
```bash
    convox env set <key=value> [key=value]... [--release <id>] [--promote] [--replace]
```

### Flags

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--id` | | bool | Output only the release ID |
| `--promote` | `-p` | bool | Promote the release after setting |
| `--release` | | string | Set variables on a specific release |
| `--replace` | | bool | Replace all environment variables instead of merging |

### Examples
```bash
    $ convox env set FOO=bar
    Setting FOO... OK
    Release: RABCDEFGHI

    $ convox env set FOO=bar --promote
    Setting FOO... OK
    Release: RABCDEFGHI
    Promoting RABCDEFGHI... OK
```

## env unset

Unset env var(s)

### Usage
```bash
    convox env unset <key> [key]... [--release <id>] [--promote]
```

### Flags

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--id` | | bool | Output only the release ID |
| `--promote` | `-p` | bool | Promote the release after unsetting |
| `--release` | | string | Unset variables on a specific release |

### Examples
```bash
    $ convox env unset FOO
    Unsetting FOO... OK
    Release: RABCDEFGHI
```

## See Also

- [Environment Variables](/configuration/environment) for environment configuration