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

```bash
    $ convox cost --app myapp
    SERVICE        GPU-HOURS  CPU-HOURS  MEM-GB-HOURS  INSTANCE     SPEND-USD
    vllm           0.00       0.00       0.00          g4dn.xlarge  $0.30
    api            0.00       0.00       0.00          t3.medium    $0.08
    worker         0.00       0.00       0.00          t3.small     $0.04
    _build         0.00       0.00       0.00          c5.large     $0.02
    _unattributed  0.00       0.00       0.00          t3.medium    $0.01
```

Each row is one entry in the underlying `AppCost.breakdown` array. The
`SPEND-USD` column is populated from the accumulator's per-service totals.
The `GPU-HOURS` / `CPU-HOURS` / `MEM-GB-HOURS` columns are reserved for a
future per-resource pricing model; in 3.24.6 they always render `0.00` and
the JSON wire fields serialize as `0` (the field tags are not `omitempty`,
so `convox cost --format json` always emits the keys with their zero
values — useful for downstream parsers that expect a stable shape).
Sort order is descending by `SPEND-USD` with alphabetical secondary tiebreak.

Two reserved buckets may appear alongside service rows:

- `_build` — build pods (carry `service-type=build`) attributed away from the
  service they are building so normal-operation cost stays uninflated.
- `_unattributed` — pods without a `service` label (system sidecars, KEDA
  scalers, anything not user-deployed).

See [Per-service cost breakdown](/management/budget-caps#per-service-breakdown)
for bucket semantics, the 1000-entry truncation cap, and the
service-rename / deleted-service / downgrade behavior.

The breakdown populates from the first accumulator tick after rack upgrade
to 3.24.6 (default tick interval is 10 minutes); pre-upgrade history is not
retroactively attributed.

### Cost tracking prerequisite

The `convox cost` read path always returns 200 — when `cost_tracking_enable`
is `false`, the response is a zero `SpendUsd` and an empty breakdown rather
than an error, so dashboards and scripts polling the endpoint do not break.
Spend only populates after the rack parameter is set:

```bash
$ convox rack params set cost_tracking_enable=true
# wait ~3 min for apply, then the next accumulator tick (~10 min default)
# starts populating spend.
```

The 422 rejection applies only to the WRITE paths — `convox budget set` and
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

- [Cost Tracking](/management/cost-tracking) — operational guide
- [Budget Caps](/management/budget-caps) — caps that consume the spend signal
- [convox.yml budget block](/configuration/convox-yml#budget) — schema reference
