---
title: "karpenter_instance_sizes"
slug: karpenter_instance_sizes
url: /configuration/rack-parameters/aws/karpenter_instance_sizes
---

# karpenter_instance_sizes

## Description

The `karpenter_instance_sizes` parameter specifies which EC2 instance sizes [Karpenter](/configuration/scaling/karpenter) can use when provisioning workload nodes.

## Default Value

The default value is empty (all sizes are allowed).

## Setting the Parameter

```bash
$ convox rack params set karpenter_instance_sizes=large,xlarge,2xlarge -r rackName
Setting parameters... OK
```

## Additional Information

- **Validation:** Comma-separated lowercase alphanumeric values (e.g., `large,xlarge,2xlarge`).
- Combine with [`karpenter_instance_families`](/configuration/rack-parameters/aws/karpenter_instance_families) to control both family and size selection.
- Build nodes use [`karpenter_build_instance_sizes`](/configuration/rack-parameters/aws/karpenter_build_instance_sizes), which falls back to this value if unset.

## See Also

- [Karpenter](/configuration/scaling/karpenter) for the full Karpenter configuration reference
- [karpenter_instance_families](/configuration/rack-parameters/aws/karpenter_instance_families)
- [karpenter_build_instance_sizes](/configuration/rack-parameters/aws/karpenter_build_instance_sizes)
