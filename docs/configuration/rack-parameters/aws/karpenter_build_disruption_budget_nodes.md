---
title: "karpenter_build_disruption_budget_nodes"
description: "The karpenter_build_disruption_budget_nodes AWS rack parameter caps how many empty Karpenter build nodes are reclaimed at once, defaulting to 100%."
slug: karpenter_build_disruption_budget_nodes
url: /configuration/rack-parameters/aws/karpenter_build_disruption_budget_nodes
---

# karpenter_build_disruption_budget_nodes

## Description

The `karpenter_build_disruption_budget_nodes` parameter caps how many empty [Karpenter](/configuration/scaling/karpenter) build nodes Karpenter reclaims in a single disruption pass. The value is a node count or a percentage of the current build node count, and a percentage is re-evaluated against the live build node count on every pass.

The value governs the `Empty` disruption reason on the build NodePool only. Drift and underutilization on the build pool stay capped at a fixed `10%` that no rack parameter exposes. The build NodePool uses the `WhenEmpty` consolidation policy, so underutilization is unreachable there in practice and the fixed cap acts as a drift cap.

Workload nodes and nodes in custom NodePools are governed by [karpenter_disruption_budget_nodes](/configuration/rack-parameters/aws/karpenter_disruption_budget_nodes) and by the per-pool `disruption_budget_nodes` field. Those settings never read this parameter, and this parameter never reads them.

This parameter applies only when [karpenter_enabled](/configuration/rack-parameters/aws/karpenter_enabled) and [build_node_enabled](/configuration/rack-parameters/aws/build_node_enabled) are both `true`. Without dedicated build nodes there is no build NodePool to configure.

## Default Value

The default value is `100%`.

`convox rack params` lists stored values only. This parameter does not appear in that output until you set it, so an absent entry means the default is in effect.

## How the Allowance Is Computed

Karpenter recomputes the allowance on every disruption pass. It resolves the budget against the current build node count, rounds a percentage up, then subtracts the build nodes that are NotReady and the ones already marked for deletion. The remainder is how many empty nodes it may reclaim in that pass, and it never drops below zero.

Build pools are small, and that is what makes the arithmetic matter. At a `10%` budget the rounded-up allowance is `1` for any pool of ten nodes or fewer, so a single NotReady or terminating build node consumes the whole allowance and empty-node reclamation stops for the entire pool, including the unhealthy node itself. At `100%` the allowance is the build node count minus the NotReady and terminating nodes, so reclamation keeps running as long as at least one build node is Ready and not already being removed.

If every build node is NotReady the allowance is zero at any budget value. The pool clears once a healthy build node joins, which happens on the next Build, or when the existing nodes reach [karpenter_node_expiry](/configuration/rack-parameters/aws/karpenter_node_expiry).

## Setting the Parameter

The default `100%` is already the fastest setting for reclaiming idle build nodes, so most Racks never set this parameter. Lower it only to slow reclamation deliberately, and keep the allowance in mind when you do: on a pool of ten nodes or fewer a `10%` value rounds to an allowance of `1`, which a single NotReady or terminating node consumes.

```bash
$ convox rack params set karpenter_build_disruption_budget_nodes=100% -r rackName
Updating parameters... OK
```

Changing the value re-renders the build NodePool disruption budgets in place. Existing build nodes are not drained or replaced.

Setting the value to `0` is accepted and deliberately freezes reclamation of empty build nodes. Empty nodes then persist until they reach `karpenter_node_expiry`.

## Additional Information

- **Availability:** AWS Racks only, and requires Rack version `3.25.3` or later. Setting it also requires a `convox` CLI at `3.25.3` or newer; an older CLI rejects the name as an unknown parameter, so run [`sudo convox update`](/reference/cli/update) first.
- **Validation:** must be a node count or a percentage from `0%` to `100%` (regex: `^((100|[0-9]{1,2})%|[0-9]+)$`). This matches the Karpenter NodePool validation, so an out-of-range value is rejected by the CLI instead of failing a Rack update. A rejected value returns `karpenter_build_disruption_budget_nodes must be a node count or a percentage from 0% to 100% (e.g. 100%)`. The CLI also rejects an empty value.
- **Behavior change on update:** a Rack that takes `3.25.3` with `karpenter_enabled=true` and `build_node_enabled=true` moves the build pool's `Empty` budget from an implicit `10%` to `100%` with no user action. Drift and expiry behavior for build nodes are unchanged, and workload and custom NodePools are untouched.
- [`karpenter_build_consolidate_after`](/configuration/rack-parameters/aws/karpenter_build_consolidate_after) sets when an empty build node becomes eligible for reclamation; this parameter sets how many go at once.
- **Parameter groups:** listed under the `karpenter`, `scaling`, and `build` groups (`convox rack params -g karpenter -r rackName`).

## See Also

- [Karpenter](/configuration/scaling/karpenter) for the full Karpenter configuration reference
- [karpenter_disruption_budget_nodes](/configuration/rack-parameters/aws/karpenter_disruption_budget_nodes) for workload node disruption
- [karpenter_build_consolidate_after](/configuration/rack-parameters/aws/karpenter_build_consolidate_after)
- [karpenter_node_expiry](/configuration/rack-parameters/aws/karpenter_node_expiry)
- [build_node_enabled](/configuration/rack-parameters/aws/build_node_enabled)
