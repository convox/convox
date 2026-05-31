---
title: "cost"
slug: cost
url: /reference/cli/cost
---
# cost

Show a per-service cost breakdown for an app: integrated GPU / CPU / memory
hours, instance type, and month-to-date spend.

### Usage
```bash
    convox cost [-a app] [--aggregate] [--start YYYY-MM-DD] [--end YYYY-MM-DD] [--format table|json]
```

`--aggregate` switches the table to a single row of app-level totals.
`--start` / `--end` bound the snapshot to a calendar window; rows whose
`AsOf` falls outside the window are zeroed. `--format json` emits the raw
`*structs.AppCost` for jq consumption.

### Examples

Default per-service / per-variant breakdown on a 3.24.6+ rack:

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
    Spot pricing applies a discount automatically when nodes are provisioned via Karpenter or an EKS spot ASG. Capacity "unknown" means the node carried neither label.
```

Aggregated app totals via `--aggregate`:

```bash
    $ convox cost --app myapp --aggregate
    APP    SPEND-USD  AS-OF        PRICING-SOURCE
    myapp  $0.45      2 minutes ago  pricing-table:2026-05
```

### Output table (3.24.6+)

The default 3.24.6 output is one row per `(service, instance-type,
capacity-type)` triple. Column-position contract:

| Position | Column | Description |
|---:|---|---|
| 1 | `SERVICE` | Service name (`_build` / `_unattributed` for reserved buckets). |
| 2 | `INSTANCE` | Instance type as labeled on the node (e.g. `g4dn.xlarge`). |
| 3 | `CAPACITY` | `on-demand`, `spot`, or `unknown` (verbatim from the node label). |
| 4 | `ACTIVE-REPLICAS` | Pod count on this variant at the most recent tick; `—` if 0. |
| 5 | `SPEND-USD` | Accumulated spend for this variant in the current billing month. |

Sort order is descending by `SPEND-USD` with alphabetical secondary
tiebreak. Each row is one entry in the underlying
`AppCost.variant-breakdown` array (`structs.ServiceVariantCostLine`).

Two reserved buckets may appear alongside service rows:

- `_build`: build pods (carry `service-type=build`) attributed away from the
  service they are building so normal-operation cost stays uninflated.
- `_unattributed`: pods without a `service` label (system sidecars, KEDA
  scalers, anything not user-deployed).

### Pre-3.24.6 racks (fallback table)

Older racks (3.24.5 and earlier) emit no `variant-breakdown` array. The
CLI auto-falls-back to the legacy aggregated columns:

```bash
    SERVICE        GPU-HOURS  CPU-HOURS  MEM-GB-HOURS  INSTANCE     SPEND-USD
    vllm           0.00       0.00       0.00          g4dn.xlarge  $0.30
```

`GPU-HOURS` / `CPU-HOURS` / `MEM-GB-HOURS` are reserved for a future
per-resource pricing model. They render `0.00` on every release that
ships this column shape. The `SPEND-USD` column is populated from the
accumulator's per-service totals.

### JSON output shape

`convox cost --format json` emits the raw `*structs.AppCost` for jq
consumption. On 3.24.6+ racks the response carries BOTH a `breakdown`
array (legacy aggregated rows) and a `variant-breakdown` array (one row
per `(service, instance-type, capacity-type)` triple). The variant
schema:

```json
{
  "service": "vllm",
  "instance-type": "g4dn.xlarge",
  "capacity-type": "on-demand",
  "spend-usd": 0.30,
  "replicas": 3
}
```

Pre-3.24.6 racks omit `variant-breakdown` entirely (the field is
`omitempty`) and emit only the legacy `breakdown` array. Stable-shape
consumers should fail-open on a missing `variant-breakdown` and prefer
that array when present.

See [Per-service cost breakdown](/management/budget-caps#per-service-breakdown)
for bucket semantics, the 1000-entry truncation cap, and the
service-rename / deleted-service / downgrade behavior.

The breakdown populates from the first accumulator tick after rack upgrade
to 3.24.6 (default tick interval is 10 minutes); pre-upgrade history is not
retroactively attributed.

### Cost tracking prerequisite

The `convox cost` read path always returns 200. When `cost_tracking_enable`
is `false`, the response is a zero `SpendUsd` and an empty breakdown rather
than an error, so dashboards and scripts polling the endpoint do not break.
Spend only populates after the rack parameter is set:

```bash
$ convox rack params set cost_tracking_enable=true
# wait ~3 min for apply, then the next accumulator tick (~10 min default)
# starts populating spend.
```

The 422 rejection applies only to the WRITE paths: `convox budget set` and
`convox deploy` against a manifest with a `budget:` block. See
[Cost tracking prerequisite](/management/budget-caps#cost-tracking-prerequisite)
for the full enable instructions. Functional scope is AWS-only today.

### Unpriced instance types

When a pod runs on an instance type the rack's price table does not know
about (a brand-new family, a custom Karpenter NodePool, an exotic GPU SKU on
metal), the row shows `?` or `0.00` for the cost columns. The pod still
runs; only the cost-tracking column is blank. See [Unpriced instance
types](/management/cost-tracking#unpriced-instance-types) for the diagnostic
recipe and workaround.

### Pricing adjustment

The `pricingAdjustment` field in `convox.yml` is applied multiplicatively at
sample time. A value of `1.10` produces 10% more recorded spend than the raw
price would; `0.95` produces 5% less. Use this to align Convox's internal
pricing with the contract pricing your finance team sees, or to add buffer
for cap headroom.

### Per-month rollover

Month-to-date spend resets to zero at the first of each month, UTC. Caps that
were tripped in the previous month are cleared as part of the rollover.

## See Also

- [Cost Tracking](/management/cost-tracking): operational guide
- [Budget Caps](/management/budget-caps): caps that consume the spend signal
- [convox.yml budget block](/configuration/convox-yml#budget): schema reference
