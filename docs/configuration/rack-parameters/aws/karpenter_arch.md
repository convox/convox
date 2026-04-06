---
title: "karpenter_arch"
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
Setting parameters... OK
```

For mixed-architecture workloads:

```bash
$ convox rack params set karpenter_arch=amd64,arm64 -r rackName
Setting parameters... OK
```

## Additional Information

- **Validation:** Must be `amd64`, `arm64`, `amd64,arm64`, or empty.
- When empty, Karpenter auto-detects the architecture from the Rack's `node_type` instance family.
- When both architectures are specified, Karpenter selects the optimal architecture based on pod requirements and instance availability.

## See Also

- [Karpenter](/configuration/scaling/karpenter) for the full Karpenter configuration reference
- [BuildArch](/configuration/app-parameters/aws/BuildArch) for architecture-aware build scheduling
- [node_type](/configuration/rack-parameters/aws/node_type) for primary node instance type
