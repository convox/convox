---
title: "Cost Tracking"
slug: cost-tracking
url: /management/cost-tracking
---
# Cost Tracking

Convox aggregates per-app spend from cloud-provider pricing data plus the rack's
in-cluster usage telemetry. Spend is the input to budget caps (see [Budget
Caps](/management/budget-caps)) and surfaces in the Console and the `convox cost`
CLI.

## How spend is computed

The rack samples each running pod's CPU, memory, and (where applicable) GPU
allocations on every accumulator tick (default 10 minutes). Each sample is
priced against the instance type the pod runs on, using a built-in price table
keyed by cloud provider and instance family. The per-tick samples are summed
across the month into the app's `CurrentMonthSpendUsd` field, surfaced through
`convox budget show` and the Console budget panel.

Pricing adjustment (`pricingAdjustment` in `convox.yml`) is applied
multiplicatively at sample time. A pricingAdjustment of `1.10` produces 10% more
recorded spend than the raw price would; `0.95` produces 5% less. Use this to
align Convox's internal pricing with the contract pricing your finance team
sees, or to add a buffer for cap headroom.

## Unpriced instance types <a id="unpriced-instance-types"></a>

The built-in price table covers the common instance families on each provider.
When a pod runs on an instance the table does not know about — a brand-new AWS
family, a Karpenter-spawned instance from a custom NodePool, or a custom GPU SKU
on metal — the rack records `0` for that sample. The pod still runs; only the
cost-tracking column is blank.

Symptoms:
- `convox cost --app myapp` shows `?` or `0.00` for some services.
- `app:budget:threshold` and `app:budget:cap` events do not fire even though
  cloud bills indicate the app should have crossed.

To diagnose:
- `convox ps --app myapp` shows the running pods.
- `kubectl get pod -n <rack>-<app> -o jsonpath='{.items[*].spec.nodeName}'` plus
  `kubectl get nodes -L node.kubernetes.io/instance-type` resolves each pod to
  its instance type.
- File the unrecognized type as an issue at the [convox/convox repo](https://github.com/convox/convox/issues).

To work around in the meantime, set a higher `pricingAdjustment` to compensate
for the under-counted instances, or move the impacted services to a node group
that uses a recognized instance family.

## Cost breakdown CLI

```bash
$ convox cost --app myapp
SERVICE  INSTANCE-TYPE  CPU      MEMORY    GPU  HOURLY-USD  MONTH-TO-DATE
web      t3.medium      0.5      512 MiB        0.0418      $18.34
api      m5.large       1.0      2 GiB          0.096       $42.10
worker   c5.xlarge      2.0      4 GiB          0.17        $74.21
TOTAL                                                       $134.65
```

Service-level numbers help identify which workload is driving spend. Use the
output to refine `monthlyCapUsd`, decide whether to opt a service out of
`atCapAction: auto-shutdown` via `neverAutoShutdown`, or scale the workload down
before cap fire.

## Per-month rollover

Spend resets to zero at the first of each month, UTC. Caps that were tripped in
the previous month are cleared as part of the rollover. Recovery banners and
flap-suppress carry-overs are cleared by the stale-annotation GC tick after one
poll interval (10 min default).

## See Also

- [Budget Caps](/management/budget-caps) — operational management of caps
- [convox.yml budget block](/configuration/convox-yml#budget) — schema reference
- [cost CLI reference](/reference/cli/cost) — command reference
