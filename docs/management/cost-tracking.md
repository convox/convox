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

## Enabling cost tracking <a id="enable"></a>

Cost tracking is gated by the rack parameter `cost_tracking_enable`, default
`false`. Without it, the cost accumulator does not run — no spend is
computed and budget enforcement (caps, alerts, auto-shutdown) cannot fire
even with a `budget:` block in `convox.yml`.

Read paths still return successfully: `convox cost` against a rack with
`cost_tracking_enable=false` returns a zero spend total and an empty
breakdown (HTTP 200), so dashboards and scripts that poll the endpoint do
not break — they just see "no data yet." Write paths, on the other hand,
reject loud: `convox budget set` and `convox deploy` against a manifest
with an enforcement-bearing `budget:` block return HTTP 422 with an
actionable message pointing at the enable command. Recovery operations
(`convox budget clear`, `convox budget reset`) remain available regardless
of cost-tracking state.

Enable on AWS racks:

```bash
$ convox rack params set cost_tracking_enable=true
```

Wait ~3 minutes for the rack apply to complete, then deploy or set budgets.
The first accumulator tick after the apply (default tick interval is 10
minutes) starts populating spend. The Console budget panel and `convox cost`
become populated from that tick onward.

`cost_tracking_enable` is **AWS-only**. Non-AWS racks (Azure, GCP,
DigitalOcean, Equinix Metal, Local) cannot enable cost tracking; their
built-in pricing tables and instance-type introspection paths only cover
AWS. Cost-tracking-dependent features (Console budget panel populated,
`convox cost`, per-service spend attribution) are AWS-only.

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

`convox cost` returns one row per service plus the reserved `_build` and
`_unattributed` buckets, sorted descending by `SPEND-USD` with alphabetical
secondary tiebreak:

```bash
$ convox cost --app myapp
SERVICE        GPU-HOURS  CPU-HOURS  MEM-GB-HOURS  INSTANCE     SPEND-USD
vllm           0.00       0.00       0.00          g4dn.xlarge  $0.30
api            0.00       0.00       0.00          t3.medium    $0.08
worker         0.00       0.00       0.00          t3.small     $0.04
_build         0.00       0.00       0.00          c5.large     $0.02
_unattributed  0.00       0.00       0.00          t3.medium    $0.01
```

The `SPEND-USD` column is populated from the accumulator's per-service totals.
The `GPU-HOURS` / `CPU-HOURS` / `MEM-GB-HOURS` columns are reserved for a
future per-resource pricing model; in 3.24.6 they always render `0.00`. App
totals are surfaced via `convox cost --aggregate` (a single-row table:
`APP | SPEND-USD | AS-OF | PRICING-SOURCE`). See the
[cost CLI reference](/reference/cli/cost) for the full flag set.

Service-level numbers help identify which workload is driving spend. Use the
output to refine `monthlyCapUsd`, decide whether to opt a service out of
`atCapAction: auto-shutdown` via `neverAutoShutdown`, or scale the workload
down before cap fire.

## Per-month rollover

Spend resets to zero at the first of each month, UTC. Caps that were tripped in
the previous month are cleared as part of the rollover. Recovery banners and
flap-suppress carry-overs are cleared by the stale-annotation GC tick after one
poll interval (10 min default).

## See Also

- [Budget Caps](/management/budget-caps) — operational management of caps
- [convox.yml budget block](/configuration/convox-yml#budget) — schema reference
- [cost CLI reference](/reference/cli/cost) — command reference
