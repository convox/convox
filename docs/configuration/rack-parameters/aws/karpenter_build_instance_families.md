---
title: "karpenter_build_instance_families"
slug: karpenter_build_instance_families
url: /configuration/rack-parameters/aws/karpenter_build_instance_families
---

# karpenter_build_instance_families

## Description

The `karpenter_build_instance_families` parameter specifies which EC2 instance families [Karpenter](/configuration/scaling/karpenter) can use when provisioning build nodes.

## Default Value

The default value is empty (falls back to [`karpenter_instance_families`](/configuration/rack-parameters/aws/karpenter_instance_families)).

## Setting the Parameter

```bash
$ convox rack params set karpenter_build_instance_families=c5,c6i -r rackName
Setting parameters... OK
```

## Additional Information

- **Validation:** Comma-separated lowercase alphanumeric values.
- The build NodePool is only created when [`build_node_enabled=true`](/configuration/rack-parameters/aws/build_node_enabled).

## See Also

- [Karpenter](/configuration/scaling/karpenter) for the full Karpenter configuration reference
- [karpenter_instance_families](/configuration/rack-parameters/aws/karpenter_instance_families)
- [karpenter_build_instance_sizes](/configuration/rack-parameters/aws/karpenter_build_instance_sizes)
