---
title: "karpenter_disruption_block_schedule"
description: "The karpenter_disruption_block_schedule AWS rack parameter opens a recurring window in which Karpenter will not replace nodes for drift or consolidation."
slug: karpenter_disruption_block_schedule
url: /configuration/rack-parameters/aws/karpenter_disruption_block_schedule
---

# karpenter_disruption_block_schedule

## Description

The `karpenter_disruption_block_schedule` parameter sets when a recurring window opens in which [Karpenter](/configuration/scaling/karpenter) will not replace nodes for drift or consolidation. [`karpenter_disruption_block_duration`](/configuration/rack-parameters/aws/karpenter_disruption_block_duration) sets how long that window stays open. The two must be set together and cleared together.

The window applies to the workload NodePool, the build NodePool, and every pool declared in [`additional_karpenter_nodepools_config`](/configuration/rack-parameters/aws/additional_karpenter_nodepools_config).

## Default Value

The default value is `""`, and no window is generated.

`convox rack params` lists stored values only. This parameter does not appear in that output until you set it, so an absent entry means no window is in effect.

## What the Window Blocks

Inside the window Karpenter will not replace a node for **drift**, meaning a new AMI or a changed NodePool or EC2NodeClass field, and it will not replace a node for **consolidation of an underutilized node**.

Two things continue inside the window:

- **Empty node reclamation.** A node carrying only daemonset pods is still removed inside the window. Reclaiming an empty node evicts nothing, and blocking it on the build pool would hold idle build nodes for the length of every window.
- **Node expiry.** A node reaching [`karpenter_node_expiry`](/configuration/rack-parameters/aws/karpenter_node_expiry) is deleted inside the window exactly as it is outside. Karpenter v1 expiry is not a budgeted disruption reason: the expiration controller deletes the NodeClaim outright, consults no budget, and provisions no replacement in advance. The window does not protect a fleet from an expiry wave.

## Setting the Parameter

```bash
$ convox rack params set karpenter_disruption_block_schedule="0 13 * * *" karpenter_disruption_block_duration=6h -r rackName
Updating parameters... OK
```

Setting either parameter to a non-empty value prints three lines to stderr:

```text
WARNING: the disruption window paces drift and consolidation, not node expiry. A node reaching karpenter_node_expiry is deleted inside the window, with no budget and no replacement waiting.
Existing nodes keep the expiry they were created with, so karpenter_node_expiry=Never reaches a node only when that node is replaced.
Set karpenter_node_expiry=Never and let the fleet roll before arming the window.
```

Clear both parameters in one command:

```bash
$ convox rack params set karpenter_disruption_block_schedule= karpenter_disruption_block_duration= -r rackName
Updating parameters... OK
```

Neither setting nor clearing the window replaces a node. The generated `budgets` list is excluded from the NodePool drift hash, so the change takes effect within one Karpenter disruption loop and moves nothing.

`convox rack params set` refuses these before submitting anything:

- **Either window parameter without the other.**
- **A schedule with a restricted day-of-month or month**, such as `0 9 1 * *` or `0 9 * 3 *`, and the `@monthly`, `@yearly` and `@annually` macros. A monthly recurrence cannot carry a stable fixed-duration window, since months run 672 to 744 hours.
- **A five-field schedule that fires more than once a day**, such as `0 * * * *`, `30 */2 * * *`, `*/5 * * * *` or `0 9-17 * * *`. The minute field must be a single number from `0` to `59` and the hour field a single number from `0` to `23`.
- **A duration that reaches the schedule's next firing**, which would block node replacement permanently. The bound is the smallest gap between two firings: under `168h` when the schedule names one weekday, as `0 13 * * SAT` does, and under `24h` otherwise, including `*`, a range like `MON-FRI` and a list like `0,6`. The bound is strict, so `@weekly` with exactly `168h` is rejected.
- **Both window parameters together with a `budgets` list inside `karpenter_config.nodePool.disruption`.** That list replaces the generated budgets, window entry included. To keep both, write the window entry into [`karpenter_config`](/configuration/rack-parameters/aws/karpenter_config) yourself.

`@hourly` is accepted. It fires 24 times a day and is bounded to a duration under `1h`, so `@hourly` with `30m` is legal and gives a window that is active for the first 30 minutes of every hour. The once-a-day restriction applies to the five-field forms only.

Setting one window parameter while the other stays empty is refused with:

```text
karpenter_disruption_block_schedule and karpenter_disruption_block_duration must be set together, and cleared together
```

## Expressing a Maintenance Window

Window times are UTC. Karpenter builds its cron with a hardcoded `TZ=UTC` and there is no time zone parameter, so an operator blocking business hours from a US Eastern perspective gets a window that shifts by an hour twice a year.

The two parameters express "block during this window", not "allow only during this window". Never rolling during business hours is direct:

```bash
$ convox rack params set karpenter_disruption_block_schedule="0 9 * * MON-FRI" karpenter_disruption_block_duration=8h -r rackName
Updating parameters... OK
```

Rolling only on Saturday between 09:00 and 13:00 UTC means blocking the rest of the week. The week is 168 hours and the allowed window is 4 of them, so 168 minus 4 gives a block of `164h`. It starts when the allowed window ends and lifts when that window opens again:

```bash
$ convox rack params set karpenter_disruption_block_schedule="0 13 * * SAT" karpenter_disruption_block_duration=164h -r rackName
Updating parameters... OK
```

Do the arithmetic before you set it. Off-by-one is silent at every layer, and `163h` would allow Saturday 08:00 to 13:00 with nothing reporting it.

## Ordering with karpenter_node_expiry

`expireAfter` is copied onto each NodeClaim when it is created and is never re-read from the NodePool, so changing `karpenter_node_expiry` does not change the expiry of nodes that already exist. Setting `karpenter_node_expiry=Never` changes the NodePool immediately, drifts it, and the replacement nodes are the ones that never expire. Until that roll finishes, the existing fleet is still on its original deadline.

Arm the window in three steps:

1. Set `karpenter_node_expiry=Never`.
2. Wait for the drift roll to replace the fleet. It is paced by [`karpenter_disruption_budget_nodes`](/configuration/rack-parameters/aws/karpenter_disruption_budget_nodes) and pre-spins each replacement.
3. Set the window.

In the other order, or in one command, the window blocks the drift roll that would have disarmed the fleet.

Step 2 is finished when no NodeClaim still carries the old expiry, so a fleet that has fully rolled reports one value:

```bash
$ convox api get /kubernetes/apis/karpenter.sh/v1/nodeclaims -r rackName \
    | jq -r '.items[].spec.expireAfter' | sort -u
Never
```

Any other value in that output is a node created before the change, and the roll is still running. The Rack's Kubernetes proxy requires the admin role, and any other role gets a 403 carrying `admin role required for kubernetes api access`.

## Troubleshooting

The day-of-week field is not validated by the CLI, so a typo such as `0 9 * * MONN` is accepted by `convox rack params set` and then fails inside Karpenter. Every disruption reason on that pool silently drops to zero while `kubectl get nodepool` still looks healthy. The only evidence is a Karpenter controller log line carrying `invariant violated, invalid cron` and the schedule. Convox runs the controller as the `karpenter` deployment in the `kube-system` namespace:

```bash
$ kubectl logs -n kube-system deployment/karpenter | grep 'invariant violated, invalid cron'
```

### Confirming the Window Armed

Read the rendered budgets on a NodePool rather than waiting to notice that nothing rolls:

```bash
$ convox api get /kubernetes/apis/karpenter.sh/v1/nodepools/workload -r rackName
```

An armed window is an entry in `spec.disruption.budgets` carrying `nodes: "0"`, the schedule and duration you set, and `reasons: ["Drifted", "Underutilized"]`. Substitute `build`, or a pool `name` from `additional_karpenter_nodepools_config`, for `workload` to check the other pools. The path begins with a leading slash and carries no `proxy/` segment, and `convox api get` cannot carry a query string.

## Additional Information

This parameter is available on AWS Racks only and requires Rack version `3.25.6` or later. Setting it also requires a `convox` CLI at `3.25.6` or newer; an older CLI rejects the name as an unknown parameter, so run [`sudo convox update`](/reference/cli/update) first.

- **Validation:** a five-field cron with a single minute, a single hour, and `*` for day-of-month and month, or one of `@hourly`, `@daily`, `@midnight` and `@weekly`. Day-of-week accepts a name, a number, a range, a list or `*`. Lowercase day names are accepted.
- **Clearable:** both window parameters accept an empty value and must be cleared in the same command.
- **Parameter groups:** listed under the `karpenter` and `scaling` groups (`convox rack params -g karpenter -r rackName`).

## See Also

- [Karpenter](/configuration/scaling/karpenter) for the full Karpenter configuration reference
- [karpenter_disruption_block_duration](/configuration/rack-parameters/aws/karpenter_disruption_block_duration) for how long the window stays open
- [karpenter_disruption_budget_nodes](/configuration/rack-parameters/aws/karpenter_disruption_budget_nodes) for how many workload nodes go at once
- [karpenter_node_expiry](/configuration/rack-parameters/aws/karpenter_node_expiry) for the node lifetime the window does not cover
- [karpenter_config](/configuration/rack-parameters/aws/karpenter_config) for disruption budgets the two window parameters do not express
- [additional_karpenter_nodepools_config](/configuration/rack-parameters/aws/additional_karpenter_nodepools_config)
