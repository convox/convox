---
title: "Budget"
slug: budget
url: /reference/primitives/app/budget
---
# Budget

A **Budget** is a per-app monthly spend cap, alert threshold, at-cap action, and persisted cap-trip recovery state. The Budget primitive ships with 3.24.6 and is rack-managed; it lives outside the per-app `convox.yml` to avoid coupling the cap value to deploy lifecycles.

## Fields

| Field | Type | Description |
|:------|:-----|:------------|
| `monthly_cap_usd` | `float` | Monthly hard cap in USD. The accumulator compares the running month-to-date spend against this value on every tick and trips the cap-state when spend exceeds it. |
| `alert_threshold_percent` | `int` | Threshold percentage (1-99) of `monthly_cap_usd` at which to fire the `:threshold` event. Defaults to 80. |
| `at_cap_action` | `string` | One of `alert-only`, `block-new-deploys`, or `auto-shutdown`. Controls what happens when spend crosses the cap. See [Budget Caps](/management/budget-caps) for the recovery flow per action. |
| `pricing_adjustment` | `float` | Multiplier applied to the rack's pricing table for the app's spend rollup. Used when the app runs on instance types not in the canonical pricing table; the multiplier scales the canonical rate. Defaults to 1.0. |
| `notify_before_minutes` | `int` | (`auto-shutdown` only) Notification window in minutes between `:armed` event firing and `:fired` shutdown enforcement. Defaults to 30. |

## State

The Budget primitive maintains two pieces of persisted state, both stored as namespace annotations on the app's Kubernetes namespace:

- `convox.com/budget-config-annotation` — the user-configured fields above plus a `last_cap_mutation_by` audit field.
- `convox.com/budget-state-annotation` — the runtime state machine: `current_month_spend_usd`, `current_month_spend_as_of`, `circuit_breaker_tripped`, `circuit_breaker_ack_by`, `alert_fired_at_threshold`, `alert_fired_at_cap`, `warning_count`.

When `at_cap_action: auto-shutdown` is set and the cap trips, a third annotation appears:

- `convox.com/budget-shutdown-state-annotation` — the auto-shutdown lifecycle state machine (ARMED → ACTIVE → RECOVERED / FAILED / EXPIRED) plus the per-service replica snapshot used for restore.

A fourth annotation, `convox.com/budget-recovery-banner-dismissed`, is written transiently when a Console user dismisses the post-recovery RECOVERED banner. The rack stale-annotation GC clears it alongside the shutdown-state annotation when the cycle's terminal state ages out (one tick interval, default 10 minutes).

## CLI Commands

### Setting a budget

```bash
$ convox budget set --monthly-cap-usd 500 --at-cap-action auto-shutdown --alert-threshold-percent 80 myapp
Setting budget for myapp... OK
```

### Showing the current budget

```bash
$ convox budget show myapp
App                    myapp
MonthlyCapUSD          $500.00
AlertThresholdPercent  80
AtCapAction            auto-shutdown
NotifyBeforeMinutes    30
PricingAdjustment      1.00
CurrentMonthSpend      $312.40
SpendAsOf              2026-04-27 13:42:18 UTC
CircuitBreakerTripped  false
```

### Raising the cap

The cap can be raised in-place without rewriting other fields:

```bash
$ convox budget cap raise --monthly-cap-usd 1000 myapp
Raising cap for myapp from $500.00 to $1000.00... OK
```

### Resetting after a cap-trip

When the breaker has tripped (spend over `monthly_cap_usd`), reset re-enables the configured at-cap-action:

```bash
$ convox budget reset myapp
Resetting budget cap for myapp... OK
```

### Dismissing the RECOVERED banner

After an auto-shutdown recovery completes (`:restored` event fires), a sticky RECOVERED banner appears in the Console. Dismiss it via:

```bash
$ convox budget dismiss-recovery myapp
Dismissing recovery banner for myapp... OK
```

### Simulating an auto-shutdown

Dry-run the auto-shutdown logic against the current cluster state without modifying replicas:

```bash
$ convox budget simulate-shutdown myapp
SIMULATION (dry-run): myapp would scale-to-0 the following services:
  - web (3 replicas → 0)
  - worker (2 replicas → 0)
  Estimated cost saved per hour: $1.84
```

## Lifecycle Events

The Budget primitive emits 13 distinct webhook event types over its lifecycle. See [Budget Caps](/management/budget-caps) for the full event reference, payload shape, and receiver migration notes.

## See Also

- [Budget Caps](/management/budget-caps) for the operator-facing setup, recovery, and troubleshooting flow
- [Cost Tracking](/management/cost-tracking) for the per-service spend rollup that feeds the Budget primitive's `current_month_spend_usd`
- [`convox budget`](/reference/cli/budget) for the full CLI command surface
