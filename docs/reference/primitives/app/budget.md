---
title: "Budget"
description: "A Budget is a per-app monthly spend cap with an alert threshold and a configurable action when the cap is reached, available starting with rack version 3.24.6."
slug: budget
url: /reference/primitives/app/budget
---
# Budget

A **Budget** is a per-app monthly spend cap with an alert threshold and a configurable action when the cap is reached. You set a Budget on an app to get early-warning alerts as spend approaches the cap, to block new deploys at the cap, or to automatically scale services to zero so spend stops. Budgets are available starting with rack version 3.24.6.

A Budget is configured per app and is independent of your `convox.yml` deploy lifecycle, so deploying a new release never overwrites a cap you have set.

## Fields

Cap-enforcement fields (`monthlyCapUsd`, `alertThresholdPercent`, `atCapAction`, `pricingAdjustment`) are set with `convox budget set` or the Console budget tab. A deploy never overwrites these values.

The auto-shutdown runtime fields (`atCapWebhookUrl`, `notifyBeforeMinutes`, `shutdownGracePeriod`, `recoveryMode`, `shutdownOrder`, `neverAutoShutdown`) are configured in the top-level `budget:` block of `convox.yml` and take effect on the next deploy. See the [convox.yml budget block](/configuration/convox-yml#budget) for the full schema reference.

The CLI flags and `convox.yml` keys use kebab-case (for example `monthly-cap-usd`).

| Field | Type | Description |
|:------|:-----|:------------|
| `monthlyCapUsd` | `float` | Monthly spend cap in USD. When the app's month-to-date spend exceeds this value, the configured `atCapAction` takes effect. |
| `alertThresholdPercent` | `float` | Percentage of `monthlyCapUsd` at which an early-warning alert fires (range `1-100` inclusive). Set to `100` to alert only at the cap; set lower for earlier warnings. Defaults to 80. |
| `atCapAction` | `string` | What happens when spend reaches the cap. One of `alert-only` (default), `block-new-deploys`, or `auto-shutdown`. See [Budget Caps](/management/budget-caps) for the recovery flow per action. |
| `atCapWebhookUrl` | `string` | Optional URL Convox sends a POST to when the cap is reached. Configured in the top-level `budget:` block of `convox.yml`. Empty means no webhook is sent. |
| `pricingAdjustment` | `float` | Multiplier applied to the app's calculated spend (range `0.1-1.5`). Use it to account for a negotiated discount or for instance types not in the standard pricing table. For example `0.7` applies a 30% discount. Defaults to 1.0. |
| `notifyBeforeMinutes` | `int` | (`auto-shutdown` only) Minutes between the auto-shutdown warning and the actual shutdown, giving you a window to raise the cap or intervene. Defaults to 30. |
| `shutdownGracePeriod` | `string` | (`auto-shutdown` only) How long services are given to shut down gracefully before being force-stopped (e.g. `30s`). Defaults to `5m`. |
| `recoveryMode` | `string` | (`auto-shutdown` only) `auto-on-reset` (default) restores services to their previous scale automatically when you run `convox budget reset`; `manual` requires you to scale services back up yourself. |
| `shutdownOrder` | `string` | (`auto-shutdown` only) `largest-cost` (default) shuts down the highest-cost services first; `newest` shuts down the most-recently-deployed services first. |
| `neverAutoShutdown` | `[]string` | (`auto-shutdown` only) Service names that should keep running through a cap event (e.g. `["api"]`). Listed services are excluded from the shutdown and continue to consume budget. |

## State

Alongside the fields you configure, Convox tracks the live state of each Budget so you can see where the app stands against its cap. This state is returned by `convox budget show` and surfaced in the Console budget tab, and includes:

- the current month-to-date spend and the time it was last calculated
- whether the cap has been reached, and which user acknowledged the breach (when applicable)
- whether the alert threshold and the cap have already fired this month

When `atCapAction: auto-shutdown` is in effect and the cap is reached, Convox also tracks the shutdown progress (armed, active, recovered, or failed) and the previous scale of each service so it can be restored on `convox budget reset`. Spend totals reset at the start of each month.

## CLI Commands

### Setting a budget

```bash
$ convox budget set --monthly-cap 500 --at-cap-action auto-shutdown --alert-at 80 myapp
Setting budget for myapp... OK
```

### Showing the current budget

```bash
$ convox budget show myapp
{
  "config": {
    "monthly-cap-usd": 500,
    "alert-threshold-percent": 80,
    "at-cap-action": "auto-shutdown",
    "pricing-adjustment": 1
  },
  "state": {
    "month-start": "2026-04-01T00:00:00Z",
    "current-month-spend-usd": 312.4,
    "current-month-spend-as-of": "2026-04-27T13:42:18Z"
  }
}
```

### Raising the cap

The cap can be raised in-place without rewriting other fields:

```bash
$ convox budget cap raise --monthly-cap-usd 1000 myapp
Raising monthly cap for myapp... OK
```

### Resetting after the cap is reached

Once spend has exceeded `monthlyCapUsd` and the cap action has taken effect, reset acknowledges the breach and re-enables the configured at-cap action (for `auto-shutdown`, this also restores services when `recoveryMode` is `auto-on-reset`):

```bash
$ convox budget reset myapp
Resetting budget for myapp... OK
```

### Dismissing the RECOVERED banner

After an auto-shutdown recovery completes (`:restored` event fires), a sticky RECOVERED banner appears in the Console. Dismiss it via:

```bash
$ convox budget dismiss-recovery myapp
Banner dismissed for myapp.
```

### Simulating an auto-shutdown

Dry-run the auto-shutdown logic against the current cluster state without modifying replicas:

```bash
$ convox budget simulate-shutdown myapp
Simulating auto-shutdown for myapp...

Eligibility:
  worker: ELIGIBLE -- replicas=2, cost=$0.40/hr
  web: EXEMPT (neverAutoShutdown)

Shutdown order (largest-cost-first):
  1. worker -- would scale to 0

Estimated savings: $0.40/hr

Status: SIMULATION COMPLETE. No changes made.
```

## Lifecycle Events

A Budget emits webhook events as it crosses the alert threshold, reaches the cap, and (for `auto-shutdown`) shuts down and recovers. See [Budget Caps](/management/budget-caps) for the full event reference and payload shape.

## See Also

- [Budget Caps](/management/budget-caps) for the operator-facing setup, recovery, and troubleshooting flow
- [Cost Tracking](/management/cost-tracking) for the per-service spend breakdown that a Budget's month-to-date total is based on
- [`convox budget`](/reference/cli/budget) for the full CLI command surface
