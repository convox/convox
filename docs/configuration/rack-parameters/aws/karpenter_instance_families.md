---
title: "karpenter_instance_families"
slug: karpenter_instance_families
url: /configuration/rack-parameters/aws/karpenter_instance_families
---

# karpenter_instance_families

## Description

The `karpenter_instance_families` parameter specifies which EC2 instance families [Karpenter](/configuration/scaling/karpenter) can use when provisioning workload nodes. Karpenter selects the optimal instance type from the allowed families based on pending pod requirements.

## Default Value

The default value is empty (all general-purpose instance families are allowed).

## Setting the Parameter

```bash
$ convox rack params set karpenter_instance_families=c5,m6i,r5 -r rackName
Setting parameters... OK
```

## Additional Information

- **Validation:** Comma-separated lowercase alphanumeric values (e.g., `c5,m6i,r5`).
- Combine with [`karpenter_instance_sizes`](/configuration/rack-parameters/aws/karpenter_instance_sizes) to further constrain instance selection.
- Build nodes use [`karpenter_build_instance_families`](/configuration/rack-parameters/aws/karpenter_build_instance_families), which falls back to this value if unset.

## See Also

- [Karpenter](/configuration/scaling/karpenter) for the full Karpenter configuration reference
- [karpenter_instance_sizes](/configuration/rack-parameters/aws/karpenter_instance_sizes)
- [karpenter_build_instance_families](/configuration/rack-parameters/aws/karpenter_build_instance_families)
