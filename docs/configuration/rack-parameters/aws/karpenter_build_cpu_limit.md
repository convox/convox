---
title: "karpenter_build_cpu_limit"
slug: karpenter_build_cpu_limit
url: /configuration/rack-parameters/aws/karpenter_build_cpu_limit
---

# karpenter_build_cpu_limit

## Description

The `karpenter_build_cpu_limit` parameter sets the maximum total vCPUs that [Karpenter](/configuration/scaling/karpenter) can provision across all build nodes.

## Default Value

The default value is `32`.

## Setting the Parameter

```bash
$ convox rack params set karpenter_build_cpu_limit=64 -r rackName
Setting parameters... OK
```

## Additional Information

- **Validation:** Must be a positive integer.
- The build NodePool is only created when [`build_node_enabled=true`](/configuration/rack-parameters/aws/build_node_enabled).

## See Also

- [Karpenter](/configuration/scaling/karpenter) for the full Karpenter configuration reference
- [karpenter_cpu_limit](/configuration/rack-parameters/aws/karpenter_cpu_limit)
- [karpenter_build_memory_limit_gb](/configuration/rack-parameters/aws/karpenter_build_memory_limit_gb)
