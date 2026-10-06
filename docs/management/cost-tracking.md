---
title: "Cost Tracking"
description: "Cost Tracking estimates per-app spend from built-in cloud price tables and pod resource requests, feeding budget caps and the convox cost CLI."
slug: cost-tracking
url: /management/cost-tracking
---
# Cost Tracking

Convox estimates each App's compute spend from built-in cloud-provider price tables and the resource requests of the App's running pods. Spend is the input to budget caps (see [Budget Caps](/management/budget-caps)) and surfaces in the Console and the `convox cost` CLI.

## Enabling cost tracking

Cost tracking is gated by the rack parameter `cost_tracking_enable`, default `false`. Without it, the cost accumulator does not run, so no spend is computed and budget enforcement (caps, alerts, auto-shutdown) cannot fire.

Reads still succeed while tracking is off. `convox cost` returns HTTP 200 with the last spend snapshot the Rack stored, so dashboards and scripts that poll the endpoint do not break. The snapshot is empty on a Rack that never tracked and can be non-zero on one that tracked earlier in the month. The CLI prints this notice above its output:

```text
Cost tracking is disabled on this rack. Values shown are the most-recent persisted snapshot and may be empty or stale. To enable: convox rack params set cost_tracking_enable=true
```

In `convox cost --format json` output, `tracking-enabled` is `true` while tracking runs and absent while it is off.

Writes that need the accumulator are rejected with HTTP 422 and a message pointing at the enable command: `convox budget cap raise`, `convox budget set` when it sets a cap, alert threshold or at-cap action, and every promote of an App whose `convox.yml` `budget:` block sets `monthlyCapUsd`, `alertThresholdPercent` or `atCapAction`, including the re-promote that `convox apps params set`, `convox apps lock` and `convox apps unlock` run. Recovery operations (`convox budget clear`, `convox budget reset`) remain available regardless of cost-tracking state.

Enable on AWS (3.24.6+), Azure (3.25.1+) or GCP (3.25.9+) Racks:

```bash
$ convox rack params set cost_tracking_enable=true
```

`convox rack params set` returns before the change is applied. Budget writes and promotes are accepted once the Rack update finishes, which rolls the Rack API. The accumulator ticks as soon as the new Rack API starts and then every 10 minutes. The first tick records a starting point, so an App's spend first appears on the second tick, about 10 minutes after the update completes. The Console budget panel and `convox cost` show spend from that tick onward.

Cost tracking is supported on these providers, priced from built-in list-price tables:

| Provider | Rack version | Price basis | Spot and preemptible nodes | Spot rate |
|---|---|---|---|---|
| [AWS](/configuration/rack-parameters/aws/cost_tracking_enable) | `3.24.6` or later | us-east-1 Linux on-demand list prices | `karpenter.sh/capacity-type=spot` (Karpenter) or `eks.amazonaws.com/capacityType=SPOT` (EKS spot node groups) | `0.30` of the on-demand rate for every instance type |
| [Azure](/configuration/rack-parameters/azure/cost_tracking_enable) | `3.25.1` or later | eastus Linux pay-as-you-go list prices | `kubernetes.azure.com/priority=spot` or `kubernetes.azure.com/scalesetpriority=spot` (AKS spot node pools) | `0.30` of the pay-as-you-go rate, except 19 GPU sizes with their own factor |
| [GCP](/configuration/rack-parameters/gcp/cost_tracking_enable) | `3.25.9` or later | us-central1 Linux list prices; us-east1, the default GCP Rack region, has the same prices | `cloud.google.com/gke-spot=true` or `cloud.google.com/gke-preemptible=true` | A factor per machine type, from a snapshot of Google's spot prices |

On GCP, [`preemptible`](/configuration/rack-parameters/gcp/preemptible) defaults to `true`, so a Rack on its default node pool is priced at the spot rate. Any other GKE node carries `cloud.google.com/gke-nodepool` and is priced on-demand.

DigitalOcean, Equinix Metal and Local Racks cannot enable cost tracking. Budget caps and enforcement (alerts, block-new-deploys, auto-shutdown) behave identically on every supported provider.

## How spend is computed

The accumulator ticks every 10 minutes. On each tick, a running pod is charged for the time since the previous tick, up to one hour, at its node's hourly rate, times the larger of its CPU and memory requests as a share of the node's allocatable capacity. A pod that requests GPUs on an instance type with GPUs is charged its share of the node's GPUs instead. The rate comes from a built-in price table keyed by the exact instance type name, such as `m5.large`, `Standard_D4s_v5` or `n2-standard-4`, so a size missing from the table is not priced even when its family is. The per-tick charges are summed across the month into the App's month-to-date spend (`current-month-spend-usd` in `convox budget show`), which also surfaces in the Console budget panel. From Rack version `3.25.10` the Rack also adds each tick's spend to a daily history; see [Daily history and date ranges](#daily-history-and-date-ranges).

The one-hour limit applies from Rack version `3.25.10` to any gap between ticks, such as the first tick after cost tracking is turned back on or a long Rack API outage, so spend for the rest of the gap is not counted.

The App's pricing adjustment, set with `convox budget set --pricing-adjustment` or in the Console, multiplies every tick's charge. A value of `1.10` records 10% more spend than the table price would; `0.95` records 5% less. Use it to align Convox's estimate with the contract pricing your finance team sees, or to add a buffer for cap headroom. The `pricingAdjustment` key in `convox.yml` is not read.

## Per-variant cost breakdown

Spend is attributed to each `(instance-type, capacity-type)` variant a service runs on across the month. A service that started the month on `g4dn.xlarge` on-demand and was Karpenter-replaced to `g4dn.xlarge` spot mid-month produces two rows in `convox cost --app myapp`: an on-demand row with the spend from before the move and no active replicas, and a spot row with the spend since the move and the current pod count. Rows are sorted by descending spend.

Active replicas are the pod count at the most recent tick, not a count over the month. A row with no active replicas, shown as a dash in the table, means pods previously ran on that variant but have since migrated or been removed; the accumulated spend for the variant is preserved through the rest of the month so the rollup reflects the actual cloud bill.

Spot rows are priced at the table entry's spot factor times its on-demand rate. The table is compiled into the Rack and cannot be edited. No AWS entry has its own factor, so AWS spot uses the `0.30` default; 19 Azure GPU sizes and every GCP machine type carry their own factor.

The pricing adjustment applies to the variant rows too. A value of `0.7` models a 30% discount from an AWS Enterprise Discount Program, Savings Plans or Reserved Instances, so Convox-reported spend tracks your contract pricing rather than the list rate. A value of `1.10` adds a 10% buffer for cap headroom.

## Unpriced instance types

The built-in price table covers the common instance families on each provider. When a pod runs on an instance type the table does not list (a brand-new AWS family, an instance type from a custom Karpenter NodePool, or a GCP custom machine type), the Rack adds no spend for it and counts the pod in `warning-count`. The pod still runs; only its cost is missing.

A node whose instance type the table prices without GPUs, but which has GPUs attached, is charged for CPU and memory only, and its pods are counted in `warning-count`. GCP node pools that attach GPUs to N1 machine types are priced this way, so their GPU cost is missing.

Symptoms:
- `convox cost --app myapp` shows less spend than expected, or no row, for some services.
- `warning-count` is above zero in `convox cost --app myapp --format json` and `convox budget show myapp`, and the Console cost views show an unpriced instance type warning.
- `app:budget:threshold` and `app:budget:cap` events do not fire even though cloud bills indicate the app should have crossed.

To diagnose:
- `convox ps --app myapp` shows the running pods.
- `kubectl get pod -n <rack>-<app> -o jsonpath='{.items[*].spec.nodeName}'` plus `kubectl get nodes -L node.kubernetes.io/instance-type` resolves each pod to its instance type.
- File the unrecognized type as an issue at the [convox/convox repo](https://github.com/convox/convox/issues).

To work around in the meantime, set a higher pricing adjustment with `convox budget set --pricing-adjustment` to compensate for the under-counted instances, or move the impacted services to a node group that uses a recognized instance family.

## Cost breakdown CLI

`convox cost` returns one row per service, instance type and capacity type, plus the reserved `_build` and `_unattributed` buckets, sorted descending by `SPEND-USD` with alphabetical secondary tiebreak:

```bash
$ convox cost --app myapp
SERVICE        INSTANCE     CAPACITY   ACTIVE-REPLICAS  SPEND-USD
vllm           g4dn.xlarge  on-demand  3                $0.30
api            t3.medium    spot       2                $0.08
worker         t3.small     spot       1                $0.04
_build         c5.large     on-demand  —                $0.02
_unattributed  t3.medium    on-demand  —                $0.01
TOTAL: $0.45
Cost accumulates per (instance-type, capacity-type) combination across the month. A row may show 0 active replicas if pods previously ran on that variant but have since migrated or been removed.
Spot pricing applies a discount automatically when nodes are provisioned via Karpenter, an EKS spot ASG, an AKS spot node pool, or a GKE Spot or preemptible node pool. Capacity "unknown" means the node carried no capacity label.
```

The `SPEND-USD` column is the accumulated spend for each variant. A Rack that has not recorded any spend yet returns no per-variant rows, and the CLI then prints a legacy table with `GPU-HOURS`, `CPU-HOURS` and `MEM-GB-HOURS` columns that always render `0.00`. App totals are surfaced via `convox cost --aggregate` (a single-row table: `APP | SPEND-USD | AS-OF | PRICING-SOURCE`). See the [cost CLI reference](/reference/cli/cost) for the full flag set.

Service-level numbers help identify which workload is driving spend. Use the output to refine the monthly cap, decide whether to opt a service out of auto-shutdown via `neverAutoShutdown`, or scale the workload down before cap fire.

## Daily history and date ranges

From Rack version `3.25.10`, the Rack keeps each App's spend per UTC day, per Service, for [`cost_tracking_history_days`](/configuration/rack-parameters/aws/cost_tracking_history_days) days, `62` by default. [`convox cost --start` and `--end`](/reference/cli/cost#date-ranges), with CLI version `3.25.10` or later, and the Console date range pickers sum the days in a range. Month-to-date spend, budget caps, alerts and auto-shutdown do not use the history.

| Behavior | Detail |
|---|---|
| Day boundaries | UTC. West of UTC, an evening's spend lands in the next UTC day |
| First day | the first tick after the Rack updates to `3.25.10`. Nothing is backfilled, and the update day holds only part of that day's spend |
| Month rollover, **Reset Period**, `convox budget clear` | reset month-to-date spend only. The history is kept, so range spend can exceed month-to-date spend |
| Service names | `_build` and `_unattributed` appear as in the month-to-date breakdown. `_other` collects any Service after the first 50 recorded on a day |
| Retention | lowering `cost_tracking_history_days` drops older days; raising it keeps more from then on |
| Downgrade below `3.25.10` | the stored days stay, but no days are recorded while the Rack runs the older version, so the range shows a gap for that period |

## Per-month rollover

Spend resets to zero at the first of each month, UTC, and caps that tripped in the previous month are cleared with it. The rollover does not restore Services that auto-shutdown scaled to zero. With `recoveryMode: auto-on-reset` they stay at zero until `convox budget reset`; with `manual`, scale them back up yourself. The 24-hour flap-suppression cooldown is not tied to the month: it ends 24 hours after the restore that started it. The daily history is not reset at rollover.

## See Also

- [Budget Caps](/management/budget-caps): operational management of caps
- [convox.yml budget block](/configuration/convox-yml#budget): schema reference
- [cost CLI reference](/reference/cli/cost): command reference
- [Budget Management](/console/budget-management): Console UI for cost and budget management
