---
title: "Config Mounts"
slug: config-mounts
url: /configuration/config-mounts
---

# Config Mounts

Config Mounts allow you to mount configuration files into your service containers as Kubernetes Secrets. Use them for configuration files that need to be managed independently from your application image, such as custom config files, certificate bundles, or application settings.

## Defining Config Mounts

Define configurations in the top-level `configs` section of your `convox.yml` and reference them in services via `configMounts`:

```yaml
configs:
  - id: app-config

services:
  web:
    build: .
    port: 3000
    configMounts:
      - id: app-config
        dir: /etc/myapp
        filename: config.json
```

### configs

The top-level `configs` section declares named configuration objects.

| Attribute | Type | Default | Description |
| --------- | ---- | ------- | ----------- |
| **id** | string | | **Required.** Unique identifier for the config, referenced by `configMounts` |
| **name** | string | | Optional name for the generated Kubernetes Secret |
| **value** | string | | The configuration value. If omitted, the config must be populated via the Convox API |

### configMounts

The `configMounts` attribute on a service specifies which configs to mount and where.

| Attribute | Type | Default | Description |
| --------- | ---- | ------- | ----------- |
| **id** | string | | **Required.** The `id` of a config defined in the `configs` section |
| **dir** | string | | **Required.** Directory inside the container where the config file is mounted |
| **filename** | string | | **Required.** Filename for the mounted config file. The file is mounted at `<dir>/<filename>` |

Config mounts are also supported on [initContainers](/reference/primitives/app/service#initcontainer) using the same syntax.

## How It Works

Each config defined in the `configs` section creates a Kubernetes Secret. When a service references a config via `configMounts`, the secret is mounted as a file inside the container at `<dir>/<filename>`. This allows you to manage configuration data separately from your application code and container image.

## See Also

- [Volumes](/configuration/volumes) for persistent and ephemeral storage options
- [Environment Variables](/configuration/environment) for key-value configuration
