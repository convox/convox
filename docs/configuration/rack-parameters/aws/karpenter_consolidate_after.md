---
title: "karpenter_consolidate_after"
slug: karpenter_consolidate_after
url: /configuration/rack-parameters/aws/karpenter_consolidate_after
---

# karpenter_consolidate_after

## Description

The `karpenter_consolidate_after` parameter sets the delay before [Karpenter](/configuration/scaling/karpenter) consolidation triggers on workload nodes.

## Default Value

The default value is `30s`.

## Setting the Parameter

```bash
$ convox rack params set karpenter_consolidate_after=5m -r rackName
Setting parameters... OK
```

## Additional Information

- **Validation:** Must be a duration like `30s`, `5m`, or `1h` (regex: `^\d+[smh]$`).
- A longer delay reduces churn from brief load dips. A shorter delay reclaims unused capacity faster.
- Build nodes have a separate consolidation delay via [`karpenter_build_consolidate_after`](/configuration/rack-parameters/aws/karpenter_build_consolidate_after).

## See Also

- [Karpenter](/configuration/scaling/karpenter) for the full Karpenter configuration reference
- [karpenter_consolidation_enabled](/configuration/rack-parameters/aws/karpenter_consolidation_enabled)
- [karpenter_build_consolidate_after](/configuration/rack-parameters/aws/karpenter_build_consolidate_after)
