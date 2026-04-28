---
title: "Budget"
slug: budget
url: /reference/primitives/app/budget
---
# Budget

A **Budget** is a per-app monthly spend cap, alert threshold, at-cap action, and persisted cap-trip recovery state. The Budget primitive ships with 3.24.6 and is rack-managed; it lives outside the per-app `convox.yml` to avoid coupling the cap value to deploy lifecycles.

## Fields

The Budget primitive's user-configurable fields are written to `convox.yml` under
the `budget` block (camelCase keys to match the established Convox manifest
convention). The wire/annotation JSON form uses kebab-case (e.g.
`monthly-cap-usd`); the CLI param form on `convox budget set` and `convox budget
cap raise` matches the wire form. See [convox.yml budget block](/configuration/convox-yml#budget)
for the full schema reference.

| Field | Type | Description |
|:------|:-----|:------------|
| `monthlyCapUsd` | `float` | Monthly hard cap in USD. The accumulator compares the running month-to-date spend against this value on every tick and trips the cap-state when spend exceeds it. |
| `alertThresholdPercent` | `float` | Threshold percentage of `monthlyCapUsd` at which the `:threshold` event is emitted (range `1-100` inclusive). Set to `100` to fire only at exact cap; set lower for early-warning alerts. Defaults to 80. |
| `atCapAction` | `string` | One of `alert-only`, `block-new-deploys`, or `auto-shutdown`. Controls what happens when spend crosses the cap. See [Budget Caps](/management/budget-caps) for the recovery flow per action. |
| `atCapWebhookUrl` | `string` | Optional URL the rack POSTs to when the cap fires. Configured at the manifest's top-level `budget:` block (the field is per-app, not per-service). Empty means no cap-fire webhook for this app. |
| `pricingAdjustment` | `float` | Multiplier applied to the rack's pricing table for the app's spend rollup. Used when the app runs on instance types not in the canonical pricing table; the multiplier scales the canonical rate. Defaults to 1.0. |
| `notifyBeforeMinutes` | `int` | (`auto-shutdown` only) Notification window in minutes between `:armed` event firing and `:fired` shutdown enforcement. Defaults to 30. |
| `shutdownGracePeriod` | `string` | (`auto-shutdown` only) Pod `terminationGracePeriodSeconds` applied to the scale-to-0 patch (e.g. `30s`). Defaults to `5m` if unset; bounds the SIGTERM-to-SIGKILL window for service pods during budget-fire shutdown. |
| `recoveryMode` | `string` | (`auto-shutdown` only) `auto-on-reset` (default) restores replicas immediately on `convox budget reset`; `manual` requires the operator to scale services back up explicitly. |
| `shutdownOrder` | `string` | (`auto-shutdown` only) `largest-cost` (default) shuts down highest-cost-per-hour services first; `newest` shuts down most-recently-deployed services first. |
| `neverAutoShutdown` | `[]string` | (`auto-shutdown` only) Service names that must remain running through a cap-fire (e.g. `["api"]`). Listed services are excluded from the shutdown plan and continue to consume budget after `:fired`. |

## State

The Budget primitive maintains two pieces of persisted state, both stored as namespace annotations on the app's Kubernetes namespace:

- `convox.com/budget-config` — the user-configured fields above (serialized in kebab-case as `monthly-cap-usd`, `alert-threshold-percent`, `at-cap-action`, `pricing-adjustment`, `notify-before-minutes`) plus a `last-cap-mutation-by` audit field.
- `convox.com/budget-state` — the runtime state machine: `current-month-spend-usd`, `current-month-spend-as-of`, `circuit-breaker-tripped`, `circuit-breaker-ack-by`, `alert-fired-at-threshold`, `alert-fired-at-cap`, `warning-count`.

When `atCapAction: auto-shutdown` is set and the cap trips, a third annotation appears:

- `convox.com/budget-shutdown-state` — the auto-shutdown lifecycle state machine (ARMED → ACTIVE → RECOVERED / FAILED / EXPIRED) plus the per-service replica snapshot used for restore.

A fourth annotation, `convox.com/budget-recovery-banner-dismissed`, is written transiently when a Console user dismisses the post-recovery RECOVERED banner. The rack stale-annotation GC clears it alongside the shutdown-state annotation when the cycle's terminal state ages out (one tick interval, default 10 minutes).

## CLI Commands

### Setting a budget

```bash
$ convox budget set --monthly-cap 500 --at-cap-action auto-shutdown --alert-at 80 myapp
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

When the breaker has tripped (spend over `monthlyCapUsd`), reset re-enables the configured at-cap-action:

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
- [Cost Tracking](/management/cost-tracking) for the per-service spend rollup that feeds the Budget primitive's `current-month-spend-usd`
- [`convox budget`](/reference/cli/budget) for the full CLI command surface
