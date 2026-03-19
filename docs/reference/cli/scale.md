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
    2026-01-15T14:54:50Z system/cloudformation aws/cfm test-nodejs UPDATE_IN_PROGRESS test-nodejs User Initiated
    2026-01-15T14:54:55Z system/cloudformation aws/cfm test-nodejs UPDATE_IN_PROGRESS ResourceDatabase
    2026-01-15T14:54:56Z system/cloudformation aws/cfm test-nodejs UPDATE_COMPLETE ResourceDatabase
    2026-01-15T14:54:59Z system/cloudformation aws/cfm test-nodejs UPDATE_IN_PROGRESS ServiceWeb
    ...
    2026-01-15T14:57:53Z system/cloudformation aws/cfm test-nodejs UPDATE_COMPLETE ServiceWeb
    2026-01-15T14:57:54Z system/cloudformation aws/cfm test-nodejs UPDATE_COMPLETE ResourceDatabase
    2026-01-15T14:57:54Z system/cloudformation aws/cfm test-nodejs UPDATE_COMPLETE test-nodejs
    OK
```

### Flags

| Flag | Description |
|------|-------------|
| `--count` | Number of desired replicas for the service |
| `--cpu` | CPU allocation in millicores (e.g., 250 = 0.25 vCPU) |
| `--memory` | Memory allocation in MB |

## See Also

- [Scaling](/configuration/scaling) for autoscaling configuration
- [Service](/reference/primitives/app/service) for service scale attributes