---
title: "scale"
slug: scale
url: /reference/cli/scale
---
# scale

The `convox scale` command views and adjusts the scale parameters of a service, including replica count, CPU allocation, and memory allocation. When called without flags, it displays the current scale configuration for all services in the app.

## scale

Scale a service

### Usage
```bash
    convox scale <service>
```
### Examples
```bash
    $ convox scale web --count 3 --cpu 250 --memory 1024
    Scaling web...
    2026-01-15T14:54:50Z system/k8s/atom/app Status: Running => Pending
    2026-01-15T14:54:51Z system/k8s/web Scaled up replica set web-745f845dc to 3
    2026-01-15T14:54:51Z system/k8s/web-745f845dc Created pod: web-745f845dc-abc12
    2026-01-15T14:54:52Z system/k8s/atom/app Status: Pending => Updating
    2026-01-15T14:54:53Z system/k8s/web-745f845dc-abc12 Pulling image "registry.0a1b2c3d4e5f.convox.cloud/myapp:web.BABCDEFGHI"
    2026-01-15T14:54:56Z system/k8s/web-745f845dc-abc12 Successfully pulled image "registry.0a1b2c3d4e5f.convox.cloud/myapp:web.BABCDEFGHI"
    2026-01-15T14:54:56Z system/k8s/web-745f845dc-abc12 Created container main
    2026-01-15T14:54:56Z system/k8s/web-745f845dc-abc12 Started container main
    2026-01-15T14:55:01Z system/k8s/atom/app Status: Updating => Running
    OK
```

### Flags

| Flag | Description |
|------|-------------|
| `--count` | Number of desired replicas for the service |
| `--cpu` | CPU allocation in millicores (e.g., 250 = 0.25 vCPU) |
| `--memory` | Memory allocation in MB |
| `--gpu` | Number of GPU devices to reserve per pod |
| `--gpu-vendor` | GPU vendor. Supported: `nvidia` (default), `amd` |

### GPU Column

When no flags are passed, `convox scale` prints a table including a `GPU` column. Services with no GPU reservation render as `-`.

```bash
    $ convox scale
    SERVICE  DESIRED  RUNNING  CPU  MEMORY  GPU
    web      1        1        256  1024    -
    vllm     1        1        4000 16384   1
```

## See Also

- [Scaling](/configuration/scaling) for autoscaling configuration
- [Service](/reference/primitives/app/service) for service scale attributes