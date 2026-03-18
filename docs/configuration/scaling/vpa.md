---
title: "Vertical Pod Autoscaler (VPA)"
slug: vpa
url: /configuration/scaling/vpa
---
# Vertical Pod Autoscaler (VPA)

> VPA is available on AWS racks.

The Vertical Pod Autoscaler automatically adjusts CPU and memory requests for your services based on observed usage. Unlike horizontal autoscaling which changes the number of replicas, VPA right-sizes each replica's resource allocation.

## Prerequisites

Enable VPA on your rack:

```bash
$ convox rack params set vpa_enable=true -r rackName
Setting parameters... OK
```

## Configuration

Define VPA settings in the `scale.vpa` section of your service in `convox.yml`:

```yaml
services:
  web:
    build: .
    port: 3000
    scale:
      count: 3
      vpa:
        updateMode: Initial
        minCpu: "100"
        maxCpu: "2000"
        minMem: "256"
        maxMem: "4096"
```

## Attributes

| Attribute | Type | Default | Description |
| --------- | ---- | ------- | ----------- |
| **updateMode** | string | | **Required.** How VPA applies recommendations: `Off`, `Initial`, or `Recreate` |
| **minCpu** | string | | Minimum CPU in millicores (e.g. `"100"`) |
| **maxCpu** | string | | Maximum CPU in millicores (e.g. `"2000"`) |
| **minMem** | string | | Minimum memory in MB (e.g. `"256"`) |
| **maxMem** | string | | Maximum memory in MB (e.g. `"4096"`) |
| **cpuOnly** | boolean | false | Only adjust CPU, leave memory unchanged |
| **memOnly** | boolean | false | Only adjust memory, leave CPU unchanged |
| **updateRequestOnly** | boolean | false | Only update resource requests, do not modify limits |

## Update Modes

- **Off**: VPA calculates recommendations but does not apply them. Use this to observe recommendations before enabling automatic adjustments.
- **Initial**: VPA sets resource requests only when pods are first created. Running pods are not restarted.
- **Recreate**: VPA applies recommendations by evicting pods when significant resource changes are needed.

> `cpuOnly` and `memOnly` cannot both be set to `true`.

## See Also

- [vpa_enable](/configuration/rack-parameters/aws/vpa_enable) rack parameter for enabling VPA
- [Autoscaling](/configuration/scaling/autoscaling) for horizontal scaling and GPU configuration
- [KEDA Autoscaling](/configuration/scaling/keda) for event-driven autoscaling
