---
title: "karpenter_node_expiry"
description: "The karpenter_node_expiry AWS rack parameter sets the maximum lifetime for the nodes in every Karpenter pool before automatic replacement, defaulting to 720h (30 days)."
slug: karpenter_node_expiry
url: /configuration/rack-parameters/aws/karpenter_node_expiry
---

# karpenter_node_expiry

## Description

The `karpenter_node_expiry` parameter sets the maximum lifetime for [Karpenter](/configuration/scaling/karpenter) nodes before they are automatically replaced. It governs the workload NodePool, the build NodePool, and the custom pools covered under [Custom NodePools](#custom-nodepools). This keeps your fleet on current AMIs without manual intervention.

## Default Value

The default value is `720h` (30 days).

## Setting the Parameter

```bash
$ convox rack params set karpenter_node_expiry=336h -r rackName
Updating parameters... OK
```

To disable automatic replacement:

```bash
$ convox rack params set karpenter_node_expiry=Never -r rackName
Updating parameters... OK
```

## How Expiry Removes a Node

Karpenter's expiration controller computes the NodeClaim's creation time plus `expireAfter` and deletes the NodeClaim. It reads no disruption budget, runs no scheduling simulation, and provisions no replacement first. Replacement capacity arrives afterwards, from the ordinary provisioner reacting to pods that have already gone Pending. There is no `Expired` disruption reason: the reasons a budget can pace are `Underutilized`, `Empty` and `Drifted`.

PodDisruptionBudgets and the pod-level `karpenter.sh/do-not-disrupt` annotation still apply to the drain. What expiry skips is the node budget in [`karpenter_disruption_budget_nodes`](/configuration/rack-parameters/aws/karpenter_disruption_budget_nodes), the pre-spun replacement, and the node-level `do-not-disrupt` annotation. The drain is graceful. The decision to start draining is not.

A fleet whose nodes were created together expires together, unpaced. There is no jitter on `expireAfter`, and every event that replaces a fleet re-synchronizes the cohort and restarts the clock.

## Changing the Value

`expireAfter` is copied onto each NodeClaim when it is created and is never re-read from the NodePool, so a new value reaches a node only when that node is replaced. Setting `karpenter_node_expiry=Never` changes the NodePool immediately and drifts it, and the replacement nodes are the ones that never expire. Until that roll finishes the existing fleet is still on its original deadline. The roll is graceful, paced by [`karpenter_disruption_budget_nodes`](/configuration/rack-parameters/aws/karpenter_disruption_budget_nodes), and pre-spins each replacement.

Arm [`karpenter_disruption_block_schedule`](/configuration/rack-parameters/aws/karpenter_disruption_block_schedule) only after that roll finishes. The window blocks drift, so arming it first blocks the roll that would have disarmed the fleet. See [Ordering With karpenter_node_expiry](/configuration/rack-parameters/aws/karpenter_disruption_block_schedule#ordering-with-karpenter_node_expiry).

## Telling Drift From Expiry

Both mechanisms replace nodes, and they leave different traces.

| | Drift | Expiry |
|---|-------|--------|
| Pacing | Bounded by [`karpenter_disruption_budget_nodes`](/configuration/rack-parameters/aws/karpenter_disruption_budget_nodes) on the workload pool, and by each other pool's own budget | Consults no budget |
| Replacement capacity | Pre-spun before the old node drains, so capacity does not dip | Provisioned afterwards, once pods have gone Pending |
| What you see | A gradual roll | Nodes taken together, and Pending pods until replacements arrive |

`expireAfter` counts from the NodeClaim's creation, so a cohort of nodes created together expires together. Sort the nodes by creation timestamp and the cohort shows as a block of near-identical ages, with the ages of the nodes that took their place showing whether the replacements arrived in a paced trickle or all at once:

```bash
$ kubectl get nodes -L karpenter.sh/nodepool --sort-by=.metadata.creationTimestamp
```

## Custom NodePools

This parameter governs the workload and build NodePools. From Rack version `3.25.6` a pool declared in [`additional_karpenter_nodepools_config`](/configuration/rack-parameters/aws/additional_karpenter_nodepools_config) with no `node_expiry` field inherits it as well. Earlier versions used a fixed `720h` on those pools regardless of the Rack value. A per-pool `node_expiry` field still overrides it.

On a Rack that has custom pools and a non-default `karpenter_node_expiry`, upgrading to `3.25.6` replaces the nodes in those pools once. That roll is graceful and paced by the pool's own `disruption_budget_nodes`. A Rack on the default `720h` sees nothing. Downgrading below `3.25.6` returns the custom pools to `720h`, which rolls them once more on a Rack running a non-default expiry.

## Additional Information

- **Validation:** Must be a duration in hours (e.g., `720h`, `336h`) or `Never`.

## See Also

- [Karpenter](/configuration/scaling/karpenter) for the full Karpenter configuration reference
- [karpenter_disruption_budget_nodes](/configuration/rack-parameters/aws/karpenter_disruption_budget_nodes)
- [karpenter_disruption_block_schedule](/configuration/rack-parameters/aws/karpenter_disruption_block_schedule) for a window that blocks drift and consolidation but not expiry
- [karpenter_disruption_block_duration](/configuration/rack-parameters/aws/karpenter_disruption_block_duration)
- [additional_karpenter_nodepools_config](/configuration/rack-parameters/aws/additional_karpenter_nodepools_config) for the per-pool `node_expiry` override
- [karpenter_ami_alias](/configuration/rack-parameters/aws/karpenter_ami_alias) for pinning the AMI replacement nodes launch from
