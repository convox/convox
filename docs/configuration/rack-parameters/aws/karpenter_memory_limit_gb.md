---
title: "karpenter_memory_limit_gb"
slug: karpenter_memory_limit_gb
url: /configuration/rack-parameters/aws/karpenter_memory_limit_gb
---

# karpenter_memory_limit_gb

## Description

The `karpenter_memory_limit_gb` parameter sets the maximum total memory (GiB) that [Karpenter](/configuration/scaling/karpenter) can provision across all workload nodes. This acts as a safety limit against runaway scaling.

## Default Value

The default value is `400`.

## Setting the Parameter

```bash
$ convox rack params set karpenter_memory_limit_gb=800 -r rackName
Setting parameters... OK
```

## Additional Information

- **Validation:** Must be a positive integer.
- This limit applies only to the workload NodePool. Build nodes have a separate limit via [`karpenter_build_memory_limit_gb`](/configuration/rack-parameters/aws/karpenter_build_memory_limit_gb).

## See Also

- [Karpenter](/configuration/scaling/karpenter) for the full Karpenter configuration reference
- [karpenter_cpu_limit](/configuration/rack-parameters/aws/karpenter_cpu_limit)
- [karpenter_build_memory_limit_gb](/configuration/rack-parameters/aws/karpenter_build_memory_limit_gb)
