---
title: "karpenter_arch"
description: "The karpenter_arch AWS rack parameter sets the CPU architecture for Karpenter workload nodes, auto-detected from node_type when left empty."
slug: karpenter_arch
url: /configuration/rack-parameters/aws/karpenter_arch
---

# karpenter_arch

## Description

The `karpenter_arch` parameter sets the CPU architecture for [Karpenter](/configuration/scaling/karpenter) workload nodes.

## Default Value

The default value is empty (auto-detected from [`node_type`](/configuration/rack-parameters/aws/node_type)).

## Setting the Parameter

```bash
$ convox rack params set karpenter_arch=arm64 -r rackName
Updating parameters... OK
```

For mixed-architecture workloads:

```bash
$ convox rack params set karpenter_arch=amd64,arm64 -r rackName
Updating parameters... OK
```

## Additional Information

- **Validation:** Must be `amd64`, `arm64`, or `amd64,arm64`. Empty is the initial default (auto-detect), not a settable value.
- Write the mixed value with no spaces: `amd64,arm64`.
- When unset, Karpenter auto-detects the architecture from the Rack's `node_type` instance family.
- When both architectures are specified, Karpenter selects the optimal architecture based on pod requirements and instance availability.
- Once set, the parameter cannot be cleared back to auto-detect; set it explicitly to the desired value instead.
- Images built by Convox are single-architecture (the architecture of the build node), so on a mixed-architecture rack, pin each such service to its image's architecture with `nodeSelectorLabels`. See [Architecture Selection and Mixed-Architecture Racks](/configuration/scaling/karpenter#architecture-selection-and-mixed-architecture-racks).

## See Also

- [Karpenter](/configuration/scaling/karpenter) for the full Karpenter configuration reference
- [BuildArch](/configuration/app-parameters/aws/BuildArch) for architecture-aware build scheduling
- [node_type](/configuration/rack-parameters/aws/node_type) for primary node instance type
