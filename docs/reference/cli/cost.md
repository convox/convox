---
title: "cost"
description: "The convox cost command shows an app's month-to-date spend by service, instance type, and capacity type, or its spend over a range of UTC days."
slug: cost
url: /reference/cli/cost
---
# cost

Show an App's month-to-date spend by Service, instance type, and capacity type, or its spend over a range of days.

### Usage
```bash
    convox cost [-a app] [--aggregate] [--start YYYY-MM-DD] [--end YYYY-MM-DD] [--format table|json]
```

`--aggregate` switches the table to a single row of app-level totals. `--start` and `--end` select a range of UTC days; see [Date ranges](#date-ranges). `--format json` prints the cost response as JSON for jq consumption.

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
    Spot pricing applies a discount automatically when nodes are provisioned via Karpenter, an EKS spot ASG, an AKS spot node pool, or a GKE Spot or preemptible node pool. Capacity "unknown" means the node carried no capacity label.
```

Aggregated app totals via `--aggregate`:

```bash
    $ convox cost --app myapp --aggregate
    APP    SPEND-USD  AS-OF          PRICING-SOURCE
    myapp  $0.45      2 minutes ago  pricing-table:2026-09-28
```

`PRICING-SOURCE` shows the date of the Rack's pricing table, which ships with the Rack version: `2026-09-28` from Rack version `3.25.9`.

### Date ranges

With Rack version `3.25.10` or later and CLI version `3.25.10` or later, `--start` and `--end` return the App's spend for a range of UTC days, summed from the daily history the Rack keeps for [`cost_tracking_history_days`](/configuration/rack-parameters/aws/cost_tracking_history_days) days, `62` by default. Both take `YYYY-MM-DD`, and both days are included.

```bash
    $ convox cost --app myapp --start 2026-11-01 --end 2026-11-30
    Range: 2026-11-01 to 2026-11-30 (UTC)
    SERVICE  INSTANCE   SPEND-USD
    web      t3.medium  $412.18
    worker   t3.small   $96.40
    _build   c5.large   $3.12
    TOTAL: $511.70
```

| Case | Output |
|---|---|
| `--start` omitted | the range starts on the first day stored for the App |
| `--end` omitted | the range ends on the current UTC day |
| No history stored yet | `No cost history stored for this app yet` above the table, and `TOTAL: $0.00` |
| History starts after `--start` | `Cost history for this app starts <day>` above the table |
| A range before the history, or in the future | `TOTAL: $0.00`, not an error |
| `--aggregate` | the single `APP` row with the range spend, and no `TOTAL` line |
| Cost tracking off | the tracking-disabled notice above the range line; the stored days are still summed |

Rows are per Service, sorted by descending spend, and the reserved `_build` and `_unattributed` buckets can appear as in the month-to-date table. `_other` collects the spend of any Service after the first 50 recorded on a given day. `INSTANCE` is the Service's instance type in the current month's breakdown, and is empty for a Service with no spend this month. The range table has no `CAPACITY` or `ACTIVE-REPLICAS` column.

History starts at the first tick after the Rack updates to `3.25.10`, and nothing earlier is backfilled. Range spend can exceed month-to-date spend, because the Console's **Reset Period** and `convox budget clear` zero month-to-date spend but not the history. The CLI rejects a `--start` after `--end`, and any date not in `YYYY-MM-DD` form.

On a Rack before `3.25.10`, the CLI prints `Date ranges need rack version 3.25.10 or later; showing the month-to-date snapshot` in table output and shows the month-to-date table. If the snapshot's last tick falls outside the range, every row and `TOTAL` show `$0.00`. A CLI before `3.25.10` reads only the month-to-date snapshot against any Rack and prints no notice; run `sudo convox update` to read the history.

### Output table (3.24.6+)

The default 3.24.6 output is one row per `(service, instance-type, capacity-type)` triple. Column-position contract:

| Position | Column | Description |
|---:|---|---|
| 1 | `SERVICE` | Service name (`_build` / `_unattributed` for reserved buckets). |
| 2 | `INSTANCE` | Instance type as labeled on the node (e.g. `g4dn.xlarge`). |
| 3 | `CAPACITY` | `on-demand`, `spot`, or `unknown`, normalized from the node's capacity label (see below). |
| 4 | `ACTIVE-REPLICAS` | Pod count on this variant at the most recent tick; `—` if 0. |
| 5 | `SPEND-USD` | Accumulated spend for this variant in the current billing month. |

Sort order is descending by `SPEND-USD` with alphabetical secondary tiebreak. Each row is one entry in the underlying `AppCost.variant-breakdown` array (`structs.ServiceVariantCostLine`).

`CAPACITY` comes from the first of these node labels that matches. Label values other than GKE's `true` are matched without regard to case.

| Node label | Value | `CAPACITY` |
|---|---|---|
| `karpenter.sh/capacity-type` | `spot` or `on-demand` | `spot` or `on-demand` |
| `eks.amazonaws.com/capacityType` | `SPOT` or `ON_DEMAND` | `spot` or `on-demand` |
| `kubernetes.azure.com/priority` | `spot` or `regular` | `spot` or `on-demand` |
| `kubernetes.azure.com/scalesetpriority` | `spot` | `spot` |
| `cloud.google.com/gke-spot` or `cloud.google.com/gke-preemptible` | `true` | `spot` |
| `cloud.google.com/gke-nodepool` | any value | `on-demand` |
| none of the above | | `unknown` |

A `spot` row is priced at the spot rate. `on-demand` and `unknown` rows are priced at the on-demand rate. GKE labels its nodes with `cloud.google.com/gke-nodepool`, so GCP rows show `spot` or `on-demand` rather than `unknown`.

Two reserved buckets may appear alongside service rows:

- `_build`: build pods (carry `service-type=build`) attributed away from the service they are building so normal-operation cost stays uninflated.
- `_unattributed`: pods without a `service` label (system sidecars, KEDA scalers, anything not user-deployed).

### Fallback table

A Rack that has not recorded any spend yet emits no `variant-breakdown` array, and the CLI then falls back to the legacy aggregated columns:

```bash
    SERVICE        GPU-HOURS  CPU-HOURS  MEM-GB-HOURS  INSTANCE     SPEND-USD
    vllm           0.00       0.00       0.00          g4dn.xlarge  $0.30
```

`GPU-HOURS` / `CPU-HOURS` / `MEM-GB-HOURS` are reserved for a future per-resource pricing model. They render `0.00` on every release that ships this column shape. The `SPEND-USD` column is populated from the accumulator's per-service totals.

### JSON output shape

`convox cost --format json` emits the raw `*structs.AppCost` for jq consumption. The response carries BOTH a `breakdown` array (legacy aggregated rows) and a `variant-breakdown` array (one row per `(service, instance-type, capacity-type)` triple). The variant schema:

```json
{
  "service": "vllm",
  "instance-type": "g4dn.xlarge",
  "capacity-type": "on-demand",
  "spend-usd": 0.30,
  "replicas": 3
}
```

`variant-breakdown` is `omitempty`, so it is absent until the Rack records spend, and the response then carries only the legacy `breakdown` array. Stable-shape consumers should fail-open on a missing `variant-breakdown` and prefer that array when present.

`tracking-enabled` is `true` while cost tracking runs and absent while it is off. `warning-count`, the number of pods the most recent tick could not fully price, is absent when it is zero.

With `--start` or `--end` on Rack version `3.25.10` or later, `spend-usd` and `breakdown` cover the range, `variant-breakdown` is absent, and three fields are added: `range-start` and `range-end`, the first and last day of the range after the defaults are applied, and `history-start`, the first day stored for the App, absent when nothing is stored. Against an older Rack the CLI prints the month-to-date response without these fields and without the notice. When the snapshot's last tick falls outside the range, it sets `spend-usd` to 0, empties `breakdown`, and zeroes the spend in each `variant-breakdown` row. Check for `range-start` to tell the two apart.

See [Per-service cost breakdown](/management/budget-caps#per-service-cost-breakdown) for bucket semantics, the 1000-entry truncation cap, and the service-rename / deleted-service / downgrade behavior.

The breakdown populates from the first accumulator tick after rack upgrade to 3.24.6 (ticks run every 10 minutes); pre-upgrade history is not retroactively attributed.

### Cost tracking prerequisite

The `convox cost` read path returns 200 whether or not cost tracking is on, so dashboards and scripts polling the endpoint do not break. When `cost_tracking_enable` is `false`, the response is the last snapshot the Rack stored: empty on a Rack that never tracked, and non-zero if the Rack tracked earlier in the month. The CLI prints this notice before the table:

```text
Cost tracking is disabled on this rack. Values shown are the most-recent persisted snapshot and may be empty or stale. To enable: convox rack params set cost_tracking_enable=true
```

Spend only accumulates while the rack parameter is set:

```bash
$ convox rack params set cost_tracking_enable=true
# wait for the Rack update to finish; spend first appears on the second
# accumulator tick after it, about 10 minutes later.
```

The Rack returns HTTP 422 only for writes that need the accumulator: `convox budget cap raise`, `convox budget set` when it sets a cap, alert threshold or at-cap action, and any promote of an App whose `budget:` block sets `monthlyCapUsd`, `alertThresholdPercent` or `atCapAction`. That covers `convox deploy`, `convox releases promote`, and the re-promote that `convox apps params set`, `convox apps lock` and `convox apps unlock` run. A `budget:` block that sets only `pricingAdjustment` or the auto-shutdown fields is not rejected. See [Cost tracking prerequisite](/management/budget-caps#prerequisite-cost-tracking-must-be-enabled) for the full enable instructions. Cost tracking is available on AWS Racks (3.24.6+), Azure Racks (3.25.1+) and GCP Racks (3.25.9+).

### Unpriced instance types

When a pod runs on an instance type the rack's price table does not list (a brand-new family, or an instance type from a custom Karpenter NodePool), the pod adds no spend and is counted in `warning-count`. The pod still runs; only its cost is missing. See [Unpriced instance types](/management/cost-tracking#unpriced-instance-types) for the diagnostic recipe and workaround.

### Pricing adjustment

The app's pricing adjustment, set with `convox budget set --pricing-adjustment` or in the Console, multiplies spend on every tick. A value of `1.10` records 10% more spend than the table price would; `0.95` records 5% less. Use this to align Convox's internal pricing with the contract pricing your finance team sees, or to add buffer for cap headroom. `pricing-adjustment` in `--format json` output shows the value in effect. The `pricingAdjustment` key in `convox.yml` is not read.

### Per-month rollover

Month-to-date spend resets to zero at the first of each month, UTC. The daily history behind `--start` and `--end` is not reset, so a range can span months. Caps that were tripped in the previous month are cleared as part of the rollover. The rollover does not restore Services that auto-shutdown scaled to zero. With `recoveryMode: auto-on-reset` they stay at zero until `convox budget reset`; with `manual`, scale them back up yourself.

## See Also

- [Cost Tracking](/management/cost-tracking): operational guide
- [Budget Caps](/management/budget-caps): caps that consume the spend signal
- [convox.yml budget block](/configuration/convox-yml#budget): schema reference
