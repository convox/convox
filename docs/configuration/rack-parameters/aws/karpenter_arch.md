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
- When unset, Karpenter auto-detects the architecture from the Rack's `node_type` instance family, so enabling Karpenter on an existing Rack keeps the architecture that Rack was already running and this parameter does not need to be set. Convox Console Karpenter install templates pre-set the parameters a Karpenter Rack needs.
- When both architectures are specified, Karpenter selects the optimal architecture based on pod requirements and instance availability.
- Once set, the parameter cannot be cleared back to auto-detect; set it explicitly to the desired value instead.
- With `karpenter_arch=amd64,arm64`, Convox Builds produce a multi-architecture image index, so Services schedule onto either architecture with no `nodeSelectorLabels` pinning.
- With a single value, Builds stay single-architecture and native to the node the build pod runs on. On a Rack with [`build_node_enabled=true`](/configuration/rack-parameters/aws/build_node_enabled), that node comes from the build pool, whose architecture follows [`build_node_type`](/configuration/rack-parameters/aws/build_node_type), falling back to [`node_type`](/configuration/rack-parameters/aws/node_type), not `karpenter_arch`. Setting `karpenter_arch=arm64` by hand on such a Rack whose `node_type` is x86 therefore moves the workload pool to arm64 while leaving the build pool on amd64. Set `build_node_type` to a Graviton type in the same change, or every new Build produces an image the workload pool cannot run and Processes fail with an exec format error. A Rack that inherits both from `node_type` is always consistent. With the default `build_node_enabled=false` there is no build pool, Builds run on workload nodes, and `build_node_type` has no effect.
- See [Architecture Selection and Mixed-Architecture Racks](/configuration/scaling/karpenter#architecture-selection-and-mixed-architecture-racks) for the full Build output matrix, including how an [`additional_karpenter_nodepools_config`](/configuration/rack-parameters/aws/additional_karpenter_nodepools_config) entry with a differing `arch` also switches the Rack into multi-architecture Builds.

## See Also

- [Karpenter](/configuration/scaling/karpenter) for the full Karpenter configuration reference
- [BuildArch](/configuration/app-parameters/aws/BuildArch) for pinning an individual App's Build to one architecture
- [build_node_type](/configuration/rack-parameters/aws/build_node_type) for the build node instance type
- [node_type](/configuration/rack-parameters/aws/node_type) for primary node instance type
