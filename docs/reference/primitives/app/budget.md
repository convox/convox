---
title: "Budget"
description: "A Budget is a per-app monthly spend cap with an alert threshold and a configurable action when the cap is reached, on Racks with cost tracking enabled."
slug: budget
url: /reference/primitives/app/budget
---
# Budget

A **Budget** is a per-app monthly spend cap with an alert threshold and a configurable action when the cap is reached. You set a Budget on an app to get early-warning alerts as spend approaches the cap, to block new deploys at the cap, or to automatically scale services to zero so spend stops. Budgets need [cost tracking](/management/cost-tracking), which is available on AWS from Rack version `3.24.6`, on Azure from `3.25.1` and on GCP from `3.25.9`.

A Budget is configured per app and is independent of your `convox.yml` deploy lifecycle, so deploying a new release never overwrites a cap you have set.

## Fields

Cap-enforcement fields (`monthlyCapUsd`, `alertThresholdPercent`, `atCapAction`, `pricingAdjustment`) are set with `convox budget set` or the Console budget tab. A deploy never overwrites these values.

The auto-shutdown runtime fields (`atCapWebhookUrl`, `notifyBeforeMinutes`, `shutdownGracePeriod`, `recoveryMode`, `shutdownOrder`, `neverAutoShutdown`) are configured in the top-level `budget:` block of `convox.yml` and take effect on the next deploy. See the [convox.yml budget block](/configuration/convox-yml#budget) for the full schema reference.

The field names below are the camelCase names used on this page and in `convox.yml`. The CLI flags and the kebab-case keys in the API and `convox budget show` differ:

| Field | `convox budget set` flag | API and `convox budget show` key |
|:------|:-------------------------|:---------------------------------|
| `monthlyCapUsd` | `--monthly-cap` (`--monthly-cap-usd` on `convox budget cap raise`) | `monthly-cap-usd` |
| `alertThresholdPercent` | `--alert-at` | `alert-threshold-percent` |
| `atCapAction` | `--at-cap-action` | `at-cap-action` |
| `pricingAdjustment` | `--pricing-adjustment` | `pricing-adjustment` |

| Field | Type | Description |
|:------|:-----|:------------|
| `monthlyCapUsd` | `float` | Monthly spend cap in USD. When the app's month-to-date spend reaches this value, the configured `atCapAction` takes effect. |
| `alertThresholdPercent` | `float` | Percentage of `monthlyCapUsd` at which an early-warning alert fires (range `1-100` inclusive). Set to `100` to alert only at the cap; set lower for earlier warnings. Defaults to 80. |
| `atCapAction` | `string` | What happens when spend reaches the cap. One of `alert-only` (default), `block-new-deploys`, or `auto-shutdown`. See [Budget Caps](/management/budget-caps) for the recovery flow per action. |
| `atCapWebhookUrl` | `string` | Optional URL Convox sends a POST to when the cap is reached. Configured in the top-level `budget:` block of `convox.yml`. Empty means no webhook is sent. |
| `pricingAdjustment` | `float` | Multiplier applied to the app's calculated spend (range `0.1-1.5`). Use it to account for a negotiated discount or for instance types not in the standard pricing table. For example `0.7` applies a 30% discount. Defaults to 1.0. |
| `notifyBeforeMinutes` | `int` | (`auto-shutdown` only) Minutes between the auto-shutdown warning and the actual shutdown, giving you a window to raise the cap or intervene. Defaults to 30. |
| `shutdownGracePeriod` | `string` | (`auto-shutdown` only) How long services are given to shut down gracefully before being force-stopped (e.g. `30s`). Defaults to `5m`. |
| `recoveryMode` | `string` | (`auto-shutdown` only) `auto-on-reset` (default) restores services to their previous scale automatically when you run `convox budget reset`; `manual` requires you to scale services back up yourself. |
| `shutdownOrder` | `string` | (`auto-shutdown` only) `largest-cost` (default) shuts down the highest-cost services first; `newest` shuts down the most-recently-deployed services first. Every eligible service is scaled to zero in the same tick, so the order only sets the sequence within it. |
| `neverAutoShutdown` | `[]string` | (`auto-shutdown` only) Service names that should keep running through a cap event (e.g. `["api"]`). Listed services are excluded from the shutdown and continue to consume budget. Agent and stateful services are always excluded. |

## State

Alongside the fields you configure, Convox tracks the live state of each Budget so you can see where the app stands against its cap. This state is returned by `convox budget show` and surfaced in the Console budget tab, and includes:

- the current month-to-date spend and the time it was last calculated
- whether the cap has been reached, and which user acknowledged the breach (when applicable)
- whether the alert threshold and the cap have already fired this month

When the at-cap action is `auto-shutdown` and the cap is reached, Convox also tracks the shutdown progress (armed, active, recovered, or failed) and the previous scale of each service so it can be restored on `convox budget reset`. Spend totals reset at the start of each month.

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
    "pricing-adjustment": 1,
    "last-cap-mutation-by": "alice"
  },
  "state": {
    "month-start": "2026-04-01T00:00:00Z",
    "current-month-spend-usd": 312.4,
    "current-month-spend-as-of": "2026-04-27T13:42:18Z",
    "alert-fired-at-threshold": "0001-01-01T00:00:00Z",
    "alert-fired-at-cap": "0001-01-01T00:00:00Z",
    "circuit-breaker-ack-at": "0001-01-01T00:00:00Z",
    "per-service-spend-usd": {
      "worker": 312.4
    },
    "per-service-instance-type": {
      "worker": "m5.large"
    },
    "per-service-spend-by-variant": {
      "worker": {
        "m5.large:on-demand": 312.4
      }
    },
    "per-service-variant-pods-last-tick": {
      "worker": {
        "m5.large:on-demand": 2
      }
    }
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
This will acknowledge the current spend and re-enable deploys for myapp. Continue? [y/N]: y
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

Configuration:
  at_cap_action: auto-shutdown
  webhook URL: https://hooks.example.com/budget
  notify_before_minutes: 30
  shutdown_grace_period: 5m0s
  shutdown_order: largest-cost
  recovery_mode: auto-on-reset

Eligibility:
  worker: ELIGIBLE -- replicas=2, cost=$0.40/hr
  web: EXEMPT (in neverAutoShutdown)

Shutdown order (largest-cost):
  1. worker -- would scale to 0

Estimated savings: $0.40/hr

Webhook payload sent (dry_run=true):
  See app:budget:auto-shutdown:simulated event in your atCapWebhookUrl webhook delivery and rack log aggregation

Status: SIMULATION COMPLETE. No changes made.
```

## Lifecycle Events

A Budget emits webhook events as it crosses the alert threshold, reaches the cap, and (for `auto-shutdown`) shuts down and recovers. See [Budget Caps](/management/budget-caps) for the full event reference and payload shape.

## See Also

- [Budget Caps](/management/budget-caps) for the operator-facing setup, recovery, and troubleshooting flow
- [Cost Tracking](/management/cost-tracking) for the per-service spend breakdown that a Budget's month-to-date total is based on
- [`convox budget`](/reference/cli/budget) for the full CLI command surface
