---
title: "karpenter_build_memory_limit_gb"
slug: karpenter_build_memory_limit_gb
url: /configuration/rack-parameters/aws/karpenter_build_memory_limit_gb
---

# karpenter_build_memory_limit_gb

## Description

The `karpenter_build_memory_limit_gb` parameter sets the maximum total memory (GiB) that [Karpenter](/configuration/scaling/karpenter) can provision across all build nodes.

## Default Value

The default value is `256`.

## Setting the Parameter

```bash
$ convox rack params set karpenter_build_memory_limit_gb=512 -r rackName
Setting parameters... OK
```

## Additional Information

- **Validation:** Must be a positive integer.
- The build NodePool is only created when [`build_node_enabled=true`](/configuration/rack-parameters/aws/build_node_enabled).

## See Also

- [Karpenter](/configuration/scaling/karpenter) for the full Karpenter configuration reference
- [karpenter_memory_limit_gb](/configuration/rack-parameters/aws/karpenter_memory_limit_gb)
- [karpenter_build_cpu_limit](/configuration/rack-parameters/aws/karpenter_build_cpu_limit)
