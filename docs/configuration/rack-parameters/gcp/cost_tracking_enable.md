---
title: "cost_tracking_enable"
description: "The cost_tracking_enable GCP rack parameter turns on the rack-side cost accumulator that powers convox cost and per-app budget caps, defaulting to false."
slug: cost_tracking_enable
url: /configuration/rack-parameters/gcp/cost_tracking_enable
---

# cost_tracking_enable

## Description
The `cost_tracking_enable` parameter turns on the Rack's cost accumulator, which estimates each App's Compute Engine spend and powers [`convox cost`](/reference/cli/cost) and per-App [budget caps](/management/budget-caps). The accumulator runs inside the Rack API. On each tick, every 10 minutes, it charges each running pod the share of its node's hourly price that the pod's CPU or memory requests reserve, whichever is larger, or its share of the node's GPUs when it requests GPUs. The result is added to the App's spend for the current calendar month, which resets on the 1st, UTC.

Cost tracking is a prerequisite for budget caps. While it is off, the Rack rejects `convox budget cap raise`, and any `convox budget set` that sets a monthly cap, alert threshold or at-cap action, with HTTP 422. It also rejects every promote of an App whose `convox.yml` `budget:` block sets `monthlyCapUsd`, `alertThresholdPercent` or `atCapAction`. The error points at this parameter.

## Default Value
The default value for `cost_tracking_enable` is `false`. A Rack that does not set it sees no change.

## Use Cases
- **Per-App spend on GCP**: See each App's estimated spend by Service in `convox cost` and the Convox Console, with no external cost tool.
- **Budget caps**: Set a monthly cap with `convox budget set <app> --monthly-cap 1000` and have the Rack alert, block new deploys, or scale Services to zero when the App's spend reaches it.

## Setting Parameters
To enable cost tracking on an existing Rack:
```bash
$ convox rack params set cost_tracking_enable=true -r rackName
Updating parameters... OK
```

The change updates the Rack API Deployment and creates no cloud resources. An App's spend first appears on the second tick after the update completes, about 10 minutes later.

To disable:
```bash
$ convox rack params set cost_tracking_enable=false -r rackName
Updating parameters... OK
```

The accumulator stops once the update has rolled the Rack API. Stored spend is kept, and `convox cost` shows the last snapshot with a notice that tracking is disabled. Deploy blocking and auto-shutdown stop with it, and Services that auto-shutdown already scaled to zero stay at zero until `convox budget reset`. A promote of an App whose `budget:` block sets `monthlyCapUsd`, `alertThresholdPercent` or `atCapAction` is rejected until the block is removed or tracking is enabled again, including the re-promote that `convox apps params set`, `convox apps lock` and `convox apps unlock` run.

## How GCP Spend Is Priced

| Item | Behavior |
|------|----------|
| Price basis | us-central1 Linux list prices. us-east1, the default GCP Rack region, has the same prices. Other regions differ, and the per-App pricing adjustment below corrects for them |
| Machine type | Read from the node's `node.kubernetes.io/instance-type` label, for example `n1-standard-2` |
| Families | E2, F1, G1, N1, N2, N2D, N4, N4A, N4D, C2, C2D, C3, C3D, C4, C4A, C4D, C4N, T2A, T2D, H3, M1, M2, M3, M4, Z3 and Z4D, and the GPU families G2, A2, A3, A4 and G4 |
| Spot and preemptible nodes | A node labeled `cloud.google.com/gke-spot=true` or `cloud.google.com/gke-preemptible=true` is priced at its machine type's spot rate, because Google prices the two the same. Every other GKE node is priced on-demand |
| Spot rates | One rate per machine type, current as of the pricing table date. Google changes spot prices as often as daily, so the estimate drifts from the live price |
| GPUs on an N1 machine type | A node pool that attaches GPUs with `gpu_type` and `gpu_count` is priced at the N1 machine type's rate only, without the GPU. Its pods are counted in `warning-count` |
| Machine types not in the table | Custom machine types (`e2-custom-*`, `n2-custom-*`), Cloud TPU machine types (`ct5lp-*`, `ct6e-*`, `tpu7x-*`) and `h4d-*` add no spend. Their pods are counted in `warning-count` |
| A4 | Google offers A4 only through reservations and DWS, not on demand. The table uses the DWS calendar-mode price |

[`preemptible`](/configuration/rack-parameters/gcp/preemptible) defaults to `true`, so a Rack running its default node pool is priced at the spot rate.

`warning-count` is the number of pods the most recent tick could not fully price. It appears in `convox cost --format json` and `convox budget show`, and the Console cost overview shows a warning for an App whose count is above zero, because its spend under-counts the bill.

Estimates use list prices, so discounts Google applies to your bill, such as committed use discounts, are not reflected. The pricing adjustment on an App's budget scales its estimate by a multiplier from `0.1` to `1.5`, which also corrects for a region priced above or below us-central1:

```bash
$ convox budget set my-app --monthly-cap 1000 --pricing-adjustment 0.8
```

## Downgrading
Downgrading below `3.25.9` removes the parameter. Parameter reconciliation deletes it from the stored values before the apply runs and prints `NOTICE: removing parameters not supported by version <version>: cost_tracking_enable` on stderr. Cost tracking stops and caps stop being enforced, and a promote of an App whose `budget:` block sets `monthlyCapUsd`, `alertThresholdPercent` or `atCapAction` is rejected until the block is removed, the same as setting the parameter to `false`.

On a Rack managed through the Console the value stays in the Console's stored parameters, so `convox rack params` keeps listing it while the Rack is on the older version even though nothing is applying it, and it is applied again when you upgrade back to `3.25.9` or later. On a self-managed Rack the value is removed from the stored parameters, so set it again after the upgrade.

## Additional Information
- The pricing table ships with the Rack version. `convox cost --aggregate` shows its date: `pricing-table:2026-09-28` from `3.25.9`.
- Budget caps, alerts and auto-shutdown work the same on GCP as on AWS and Azure. See [Budget Caps](/management/budget-caps) and [Cost Tracking](/management/cost-tracking).

## Related Parameters
- [preemptible](/configuration/rack-parameters/gcp/preemptible): Nodes in the default pool are preemptible by default and priced at the spot rate.
- [additional_node_groups_config](/configuration/rack-parameters/gcp/additional_node_groups_config): A pool with `capacity_type: SPOT` is priced at the spot rate. Cloud TPU pools are not priced.
- [webhook_signing_key](/configuration/rack-parameters/gcp/webhook_signing_key): Webhook deliveries of budget events carry an HMAC signature when this is set, so receivers can verify authenticity.

## Version Requirements
This parameter requires at least Convox rack version `3.25.9`. A CLI older than `3.25.9` rejects it on a GCP Rack with `unknown parameter 'cost_tracking_enable' for gcp provider`. A `3.25.9` CLI or the Console accepts it for an older Rack, and the Rack removes it during the update with the `NOTICE` line above, so it has no effect.
