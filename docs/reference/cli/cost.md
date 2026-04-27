---
title: "cost"
slug: cost
url: /reference/cli/cost
---
# cost

Show a per-service cost breakdown for an app: instance type, requested CPU /
memory / GPU, hourly rate, and month-to-date spend.

### Usage
```bash
    convox cost [-a app]
```

### Examples

```bash
    $ convox cost --app myapp
    SERVICE  INSTANCE-TYPE  CPU      MEMORY    GPU  HOURLY-USD  MONTH-TO-DATE
    web      t3.medium      0.5      512 MiB        0.0418      $18.34
    api      m5.large       1.0      2 GiB          0.096       $42.10
    worker   c5.xlarge      2.0      4 GiB          0.17        $74.21
    TOTAL                                                       $134.65
```

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
