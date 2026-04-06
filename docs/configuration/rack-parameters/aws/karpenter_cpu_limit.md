---
title: "karpenter_cpu_limit"
slug: karpenter_cpu_limit
url: /configuration/rack-parameters/aws/karpenter_cpu_limit
---

# karpenter_cpu_limit

## Description

The `karpenter_cpu_limit` parameter sets the maximum total vCPUs that [Karpenter](/configuration/scaling/karpenter) can provision across all workload nodes. This acts as a safety limit against runaway scaling.

## Default Value

The default value is `100`.

## Setting the Parameter

```bash
$ convox rack params set karpenter_cpu_limit=200 -r rackName
Setting parameters... OK
```

## Additional Information

- **Validation:** Must be a positive integer.
- This limit applies only to the workload NodePool. Build nodes have a separate limit via [`karpenter_build_cpu_limit`](/configuration/rack-parameters/aws/karpenter_build_cpu_limit).
- Custom NodePools created via [`additional_karpenter_nodepools_config`](/configuration/rack-parameters/aws/additional_karpenter_nodepools_config) have their own per-pool `cpu_limit`.

## See Also

- [Karpenter](/configuration/scaling/karpenter) for the full Karpenter configuration reference
- [karpenter_memory_limit_gb](/configuration/rack-parameters/aws/karpenter_memory_limit_gb)
- [karpenter_build_cpu_limit](/configuration/rack-parameters/aws/karpenter_build_cpu_limit)
