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
    convox env [--release <id>] [--reveal]
```

### Flags

| Flag | Type | Description |
|------|------|-------------|
| `--release` | string | Show environment from a specific release |
| `--reveal` | bool | Show unmasked values even on a TTY. Ignored for keys not in the mask list |

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

### Masking Sensitive Values

Keys added to the per-app mask list (via `convox env mask set`) render as `****` in terminal output. Piped output always shows real values, so backup and automation flows (`convox env > backup.env`, `convox env | grep FOO`) continue to work without flags. Use `--reveal` on a TTY for one-off inspection when the real value is needed.

```bash
    $ convox env mask set API_TOKEN -a my-app
    Setting masked env keys API_TOKEN... OK

    $ convox env -a my-app
    API_TOKEN=****
    FOO=bar

    $ convox env --reveal -a my-app
    API_TOKEN=sk-live-abcdef123456
    FOO=bar

    $ convox env -a my-app > backup.env    # pipe output is always unmasked
```

Masking affects display only. The stored value in the release record and in Kubernetes pod specs is unchanged — no `ReleasePromote` is triggered when setting or unsetting the mask list.

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

## env mask

List currently masked env keys for the app.

### Usage
```bash
    convox env mask
```

### Examples
```bash
    $ convox env mask -a my-app
    API_TOKEN
    DB_URL
```

Prints one key per line, sorted. Prints nothing when the mask list is empty.

## env mask set

Mark one or more environment variable keys as masked. Masked keys render as `****` in `convox env` and `convox releases info` output on a TTY.

### Usage
```bash
    convox env mask set <key> [key]...
```

### Examples
```bash
    $ convox env mask set API_TOKEN DB_URL -a my-app
    Setting masked env keys API_TOKEN, DB_URL... OK
```

### Validation

| Rule | Behavior on violation |
|------|-----------------------|
| Keys with whitespace or `=` | Rejected with `invalid env key name` |
| Keys with ASCII control characters | Rejected to prevent terminal-escape poisoning |
| Arguments containing `=` | Rejected with a pointer to `convox env set KEY=VALUE` |
| More than 500 keys in one list | Rejected with a size guard |

`convox env mask set` does not validate that the keys exist as env vars on the app. Masking is applied by key name whenever an env list is rendered, so keys can be set-masked before or after they are added to the environment.

### Rack Version Requirements

Masking is stored per-app via the rack's `AppConfig` endpoint. Racks without `AppConfig` support (V2 racks, or V3 racks older than 3.19.7) print a warning (`Warning: this rack version may not support env masking`) and exit successfully. On those racks, `convox env` output is unchanged.

## env mask unset

Remove one or more keys from the mask list. Keys not currently in the list are silently ignored.

### Usage
```bash
    convox env mask unset <key> [key]...
```

### Examples
```bash
    $ convox env mask unset DB_URL -a my-app
    Unsetting masked env keys DB_URL... OK
```

## See Also

- [Environment Variables](/configuration/environment) for environment configuration
- [releases info](/reference/cli/releases#releases-info) for the `--reveal` flag on release records