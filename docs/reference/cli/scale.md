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
    convox scale [<service>]
```

The positional `<service>` is required for imperative changes (`--count`,
`--cpu`, `--memory`, `--gpu`, `--gpu-vendor`, `--min`, `--max`) and optional in
read-mode. With no service argument, `convox scale` prints the full scale table
for the app. With a service argument, the table is filtered to that service's
row only — the same columns, just one row.

```bash
    $ convox scale web -a myapp
    SERVICE  DESIRED  RUNNING  CPU  MEMORY  GPU  MIN  MAX  STATUS
    web      2        2        256  1024    -    2    2    
```

If the supplied service does not exist in the app, `convox scale <name>` exits
non-zero with `service "<name>" not found in app <app>`. The check fires once
before the watch loop starts, so a typo combined with `--watch-interval` does
not loop forever printing the same error.

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
| `--min` | Minimum replica count when autoscale is configured (3.24.6+) |
| `--max` | Maximum replica count when autoscale is configured (3.24.6+) |

### Output Table

When no flags are passed, `convox scale` prints a table of the configured scale for every service in the app. Columns 1-6 (`SERVICE`, `DESIRED`, `RUNNING`, `CPU`, `MEMORY`, `GPU`) match the 3.24.5 layout exactly so existing scripts that parse the output positionally with `awk` or `cut` continue to work unchanged. 3.24.6 appends `MIN`, `MAX`, an optional `AUTOSCALE`, and a trailing `STATUS` column at positions 7+.

```bash
    $ convox scale
    SERVICE  DESIRED  RUNNING  CPU   MEMORY  GPU  MIN  MAX  AUTOSCALE    STATUS
    web      2        2        256   1024    -    2    2    -            
    vllm     0        0        4000  16384   1    0    10   gpu-util>70  COLD (~2-5m first req)
```

The `vllm` row above is at rest with zero replicas — the cold-start hint
(`COLD (~2-5m first req)`) only renders when both `DESIRED=0` and `RUNNING=0`
on a service whose autoscale `min` is 0 (a `min: 0` service that has scaled
back down to zero). Once a request triggers the first replica, `DESIRED` and
`RUNNING` become 1+ and the STATUS cell clears.

The `AUTOSCALE` column appears between `MAX` and `STATUS` when at least one service in the app has autoscaling enabled. Its cell summarizes the configured trigger (e.g. `cpu>70`, `gpu-util>80 queue>10`) — services without autoscale render `-` in that column.

The trailing `STATUS` column carries the cold-start hint (`COLD (~2-5m first req)` for services with `min: 0` autoscale) and, when the app's budget cap is breached, the per-service sub-state token (`armed-Nm`, `at-cap-keda`, `at-cap-auto`, `at-cap`).

#### Column-Position Contract

| Position | Header | Source | Stable since |
|---:|--------|--------|--------------|
| 1 | `SERVICE` | service name | 3.24.5 |
| 2 | `DESIRED` | configured replica count (`s.Count`) | 3.24.5 |
| 3 | `RUNNING` | live process count (from `ProcessList`) | 3.24.5 |
| 4 | `CPU` | CPU allocation in millicores | 3.24.5 |
| 5 | `MEMORY` | memory allocation in MB | 3.24.5 |
| 6 | `GPU` | GPU count per pod (or `-`) | 3.24.5 |
| 7 | `MIN` | min replica count from `scale.autoscale` (or `-`) | 3.24.6 |
| 8 | `MAX` | max replica count from `scale.autoscale` (or `-`) | 3.24.6 |
| 9 (optional) | `AUTOSCALE` | configured trigger summary | 3.24.6 |
| trailing | `STATUS` | cold-start hint and budget sub-state | 3.24.6 |

## See Also

- [Scaling](/configuration/scaling) for autoscaling configuration
- [Service](/reference/primitives/app/service) for service scale attributes