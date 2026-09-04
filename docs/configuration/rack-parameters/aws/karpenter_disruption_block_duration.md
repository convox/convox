---
title: "karpenter_disruption_block_duration"
description: "The karpenter_disruption_block_duration AWS rack parameter sets how long the Karpenter disruption window stays open once its schedule fires."
slug: karpenter_disruption_block_duration
url: /configuration/rack-parameters/aws/karpenter_disruption_block_duration
---

# karpenter_disruption_block_duration

## Description

The `karpenter_disruption_block_duration` parameter sets how long the [Karpenter](/configuration/scaling/karpenter) disruption window stays open once [`karpenter_disruption_block_schedule`](/configuration/rack-parameters/aws/karpenter_disruption_block_schedule) fires it. The two must be set together and cleared together.

Inside the window Karpenter will not replace a node for drift or for consolidation of an underutilized node. Empty node reclamation and node expiry continue. See [karpenter_disruption_block_schedule](/configuration/rack-parameters/aws/karpenter_disruption_block_schedule) for what the window covers, the worked maintenance-window examples, and the ordering with [`karpenter_node_expiry`](/configuration/rack-parameters/aws/karpenter_node_expiry).

## Default Value

The default value is `""`, and no window is generated.

`convox rack params` lists stored values only. This parameter does not appear in that output until you set it, so an absent entry means no window is in effect.

## Setting the Parameter

```bash
$ convox rack params set karpenter_disruption_block_schedule="0 9 * * MON-FRI" karpenter_disruption_block_duration=8h -r rackName
Updating parameters... OK
```

The duration must be shorter than the gap between two firings of the schedule, because a window that reaches the next firing never lifts. The bound is strict, so `@weekly` with exactly `168h` is rejected.

| Schedule | Duration must be under |
|----------|------------------------|
| `@hourly` | `1h` |
| `@weekly`, or a five-field schedule naming a single weekday such as `0 13 * * SAT` | `168h` |
| `@daily`, `@midnight`, and every other five-field schedule, including `*`, a range like `MON-FRI` and a list like `0,6` | `24h` |

The format takes hours and minutes and nothing else. `8h30m0s` is rejected for the seconds, and `30m8h` is rejected because minutes come before hours, so write the value as `8h`, `90m` or `8h30m`.

A value in any other shape is refused before anything is submitted:

```text
karpenter_disruption_block_duration must be a duration in hours and minutes, for example 8h, 90m or 8h30m
```

## Additional Information

This parameter is available on AWS Racks only and requires Rack version `3.25.6` or later. Setting it also requires a `convox` CLI at `3.25.6` or newer; an older CLI rejects the name as an unknown parameter, so run [`sudo convox update`](/reference/cli/update) first.

- **Validation:** hours, minutes, or hours followed by minutes, matching the regex `^([0-9]+h([0-9]+m)?|[0-9]+m)$`. The value must be positive and shorter than the schedule's firing gap.
- **Clearable:** both window parameters accept an empty value and must be cleared in the same command.
- **Parameter groups:** listed under the `karpenter` and `scaling` groups (`convox rack params -g karpenter -r rackName`).

## See Also

- [Karpenter](/configuration/scaling/karpenter) for the full Karpenter configuration reference
- [karpenter_disruption_block_schedule](/configuration/rack-parameters/aws/karpenter_disruption_block_schedule) for when the window opens and what it covers
- [karpenter_disruption_budget_nodes](/configuration/rack-parameters/aws/karpenter_disruption_budget_nodes) for how many workload nodes go at once
- [karpenter_node_expiry](/configuration/rack-parameters/aws/karpenter_node_expiry) for the node lifetime the window does not cover
- [karpenter_config](/configuration/rack-parameters/aws/karpenter_config) for disruption budgets the two window parameters do not express
