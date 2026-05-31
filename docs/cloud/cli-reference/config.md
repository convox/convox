---
title: "Configuration Management"
slug: config
url: /cloud/cli-reference/config
---

# Configuration Management

### config get

Get a configuration value.

```bash
$ convox cloud config get <name> -a <app> -i <machine>
```

### config set

Set a configuration value.

```bash
$ convox cloud config set <name> -a <app> -i <machine>
```

**Options:**
- `--file`: Configuration file
- `--restart`: Restart after setting
- `--value`: Configuration value

### configs

List configurations.

```bash
$ convox cloud configs -a <app> -i <machine>
```

**Options:**
- `--watch`: Watch for updates
