---
title: "Budget Caps"
description: "Budget Caps track per-app cloud spend and enforce a monthly cap that can alert, block new deploys, or auto-shutdown services when reached."
slug: budget-caps
url: /management/budget-caps
---
# Budget Caps

Convox tracks per-app cloud spend and lets you enforce a monthly cap. When the cap is reached, the rack can alert, block new deploys, or automatically shut services down to prevent overrun. Caps are managed at runtime with the `convox budget` CLI or the Console budget tab. The [budget block in convox.yml](/configuration/convox-yml#budget) is validated when the App builds, but it does not set runtime cap values. Set caps with `convox budget set` so cap changes are explicit, audit-attributed, and not implicitly overwritten by the next deploy.

This page is the operational guide for managing caps in production. For schema details see the [convox.yml budget block](/configuration/convox-yml#budget); for how spend is computed see [Cost Tracking](/management/cost-tracking).

## Prerequisite: cost tracking must be enabled

Budget enforcement (`monthlyCapUsd`, `alertThresholdPercent`, `atCapAction`) requires the rack-level cost accumulator. Without it, no spend is computed, so caps and alerts persist as config but never trip. Enable it on AWS (3.24.6+), Azure (3.25.1+) or GCP (3.25.9+) Racks:

```bash
$ convox rack params set cost_tracking_enable=true
```

While `cost_tracking_enable` is `false`, the Rack rejects `convox budget cap raise`, a `convox budget set` that sets a cap, alert threshold or at-cap action, and a promote of a manifest whose `budget:` block sets `monthlyCapUsd`, `alertThresholdPercent` or `atCapAction`, with HTTP 422 and a message pointing at this command. Set the rack parameter, wait for the Rack update to finish, then redeploy or retry.

`cost_tracking_enable` is supported on AWS (Rack 3.24.6+), Azure (Rack 3.25.1+) and GCP (Rack 3.25.9+); DigitalOcean, Equinix Metal and Local Racks cannot enforce budgets. Recovery operations (`convox budget clear`, `convox budget reset`) remain available regardless of cost tracking state so you can always clean up.

## Set a monthly cap

Set a cap for an app with `convox budget set`:

```bash
$ convox budget set myapp --monthly-cap 500
```

The cap value is in USD. You can configure the alert threshold, the at-cap action, and a pricing adjustment in the same command:

```bash
$ convox budget set myapp --monthly-cap 1000 --alert-at 75 --at-cap-action block-new-deploys --pricing-adjustment 0.7
```

| Flag | Default | Effect |
|---|---|---|
| `--monthly-cap` | none | Monthly spend cap in USD. Setting it enables enforcement for the app. |
| `--alert-at` | `80` | Percent of the cap at which an alert fires (the threshold alert). |
| `--at-cap-action` | `alert-only` | What happens when spend crosses the cap. See [Cap actions](#cap-actions). |
| `--pricing-adjustment` | `1.0` | Multiplier applied to computed spend (e.g. `0.7` to model a 30% committed-use discount). |

The same keys in the [convox.yml budget block](/configuration/convox-yml#budget) do not set these values; the Rack enforces only what `convox budget set` or the Console saves.

## Cap actions

The at-cap action set with `--at-cap-action` selects what happens when an app's month-to-date spend reaches its cap:

| Action | Behavior |
|--------|----------|
| `alert-only` (default) | Fires the `app:budget:cap` event and webhooks. No deploy or runtime impact. |
| `block-new-deploys` | Fires `app:budget:cap` and rejects deploys, promotes, `convox scale` and `convox run` with HTTP 409 and an over-cap error. Running services keep running. |
| `auto-shutdown` | Starts the auto-shutdown countdown. After `notifyBeforeMinutes`, every eligible service is scaled to zero in the same tick, in `shutdownOrder` order. Agent and stateful services are exempt and keep running. |

The cap check and the shutdown both run on the 10-minute accumulator tick: the countdown starts at the first tick that sees spend at or above the cap, and services scale to zero at the first tick at or after the end of `notifyBeforeMinutes`.

When you choose `auto-shutdown`, the CLI prints a warning reminding you to configure your at-cap webhook and to validate the configuration first:

```bash
$ convox budget set myapp --monthly-cap 500 --at-cap-action auto-shutdown
```

Run `convox budget simulate-shutdown myapp` to preview which services would be scaled to zero, and in what order, without actually shutting anything down. The preview sends an `app:budget:auto-shutdown:simulated` event to the Rack's webhooks, and its per-service cost is month-to-date spend divided by the hours since the month started, not the current rate.

### Choosing an action

- Use `alert-only` while you are still learning what an app costs. You get the threshold alert and the at-cap event without any runtime impact.
- Use `block-new-deploys` to stop new deploys from adding more cost without touching what is already running. Good for non-production apps that should not grow further this month.
- Use `auto-shutdown` for apps where stopping the spend matters more than staying up. Pair it with a configured webhook so your team is paged on the `:armed` event before services scale down.

## Raising or recovering a cap

Raising the cap mid-month re-enables blocked deploys (when current spend is below the new cap) and dismisses any active recovery banner:

```bash
$ convox budget cap raise myapp --monthly-cap-usd 500
Raising monthly cap for myapp... OK
```

`budget cap raise` is an alias for `budget set --monthly-cap`. Both `--monthly-cap-usd` and `--monthly-cap` are accepted.

The breaker clears only when the new cap is above both the previous cap and current spend. A cap below current spend is saved anyway: the CLI prints a warning that the cap will trip on the next accumulator tick, and blocked deploys stay blocked. Use `convox cost --app myapp` to confirm current spend before raising.

Auto-shutdown never blocks deploys. After it has fired, raising the cap does **not** restart the services it scaled to zero. Run `convox budget reset` (see below) right after the raise to restore them.

## Reset and force-clear cooldown

`convox budget reset` acknowledges a cap breach and re-enables deploys. After auto-shutdown has fired, the plain reset also restarts services that were scaled to zero. The cap value itself is unchanged.

```bash
$ convox budget reset myapp
This will acknowledge the current spend and re-enable deploys for myapp. Continue? [y/N]: y
Resetting budget for myapp... OK
```

The command asks for confirmation and refuses to prompt when stdin is not a terminal. Pass `-f` or `--force` to skip the prompt, for example in a script.

By default, reset preserves the flap-suppression cooldown: an app that recently breached, was reset, then breached again will not flip-flop into repeated auto-shutdown cycles within 24 hours.

`--force-clear-cooldown` is additive. It re-enables deploys and restores services exactly as the plain reset does, and additionally clears the flap-suppression cooldown so the next cap breach is not suppressed by the 24-hour window. Use it only when you are sure the underlying cause is resolved.

```bash
$ convox budget reset myapp --force-clear-cooldown
This will acknowledge the current spend and re-enable deploys for myapp. Continue? [y/N]: y
Resetting budget for myapp... OK
```

Reset does not change month-to-date spend. The Console's **Reset Period** button does everything a reset does and clears the cooldown, and it also zeroes the App's spend and restarts the period at the current time. The CLI has no equivalent.

## Block-new-deploys recovery

When the at-cap action is `block-new-deploys` and the cap is reached, deploys, `convox scale` and `convox run` are rejected with an over-cap error similar to:

```text
budget cap exceeded for app myapp: spent $268.42 of $250.00 cap this month
```

To recover:

1. Run `convox cost --app myapp` to confirm current spend.
2. Either raise the cap above current spend (`convox budget cap raise myapp --monthly-cap-usd NEW`) or wait for the next month rollover (current spend resets on the 1st).
3. Or accept the cap and run `convox budget reset` to re-enable deploys without raising the cap. Reset clears the cap alert along with the breaker, so while spend is still at or above the cap, the next accumulator tick, within 10 minutes, fires `app:budget:cap` and blocks deploys again. Reset alone opens a window of at most 10 minutes.

## What you see when a cap is hit

The `convox ps` `STATUS` column and the `convox services` `BUDGET` column both show a per-service sub-state token when an app's budget cap is breached or arming. The vocabulary is the same on both surfaces:

| Token | Meaning |
|-------|---------|
| `armed-Nm` | Auto-shutdown countdown is active; `N` minutes remain until services scale to zero (e.g. `armed-25m`). |
| `at-cap-keda` | Service has been scaled to zero by auto-shutdown (KEDA-managed services). |
| `at-cap-auto` | Service has been scaled to zero by auto-shutdown (deployment-only services). |
| `at-cap` | Cap is breached with `atCapAction: block-new-deploys`; no scale-to-zero, deploys are rejected. |

When you recover (raise the cap or reset), these tokens clear and any recovery banner in the Console is dismissed.

## Authorization

Budget operations split across two authorization tiers. Cap mutation, clearing budget config, and force-clearing the cooldown require the Admin role. A threshold-only change through the Rack API, plain reset, dismiss-recovery, and the read-only simulate-shutdown preview require the read-write (`rw`) role. The split applies on every provider (AWS, Azure, GCP, DigitalOcean, Equinix Metal, Local).

| Operation | Required role |
|---|---|
| Set or raise the monthly cap (`--monthly-cap`) | Admin |
| Set the at-cap action (`--at-cap-action`) | Admin |
| Set the pricing adjustment (`--pricing-adjustment`) | Admin |
| Set the alert threshold with `convox budget set --alert-at`, which also needs `--monthly-cap` | Admin |
| Change only `alert-threshold-percent` through the Rack API | `rw` |
| Clear budget config (`convox budget clear`) | Admin |
| Reset (`convox budget reset`) | `rw` |
| Reset with `--force-clear-cooldown` | Admin |
| Reset the billing period (`reset_period` through the Rack API) | `rw` |
| Dismiss recovery banner (`convox budget dismiss-recovery`) | `rw` |
| Simulate shutdown (`convox budget simulate-shutdown`, read-only) | `rw` |

The Admin requirement on cap changes, clear, and force-clear keeps an admin-set cap from being circumvented by a non-admin (for example, clearing the budget and re-setting it without the cap).

Basic-auth callers (using the rack password) pass the Admin check automatically. A non-Admin Console user who attempts an Admin-only action sees the rejection message relayed through the Console UI. The exact rejection messages are:

```text
403 AppBudgetSet: admin role required to set budget cap
403 AppBudgetClear: admin role required to remove budget config
403 AppBudgetReset --force-clear-cooldown requires Admin role; current role is 'w'. Contact rack admin or use Admin token.
```

## Per-service cost breakdown

`convox cost --app myapp --format json` returns a `breakdown` array with the cumulative spend, instance type, and bucket name for every service that has been observed running this month. The breakdown grows over the month and resets to zero at month rollover alongside the month-to-date spend.

Two reserved buckets appear alongside service names:

- `_build`: spend for build pods, bucketed away from the service being built so the named service's normal-operation cost stays uninflated.
- `_unattributed`: spend for pods with no service label (system sidecars, autoscaler components, anything not user-deployed). Kept visible without inflating any user-deployed service's row.

Edge cases:

- **Service deleted mid-month.** The deleted service's accumulated spend remains in the breakdown until month rollover. You ran the service for part of the month, so its cost stays attributed for that month.
- **Service renamed mid-month.** The old name keeps its pre-rename spend; the new name accumulates from the rename point forward. Both rows appear until rollover and sum to the correct app total.
- **Per-app entry cap.** The breakdown holds up to 1000 services per app. Once full, existing rows keep accumulating but new services are dropped from this month's breakdown, and an `app:budget:per-service-truncated` event fires. Subscribe via webhook to surface it. Most apps stay well under the cap; if you hit it, check for unbounded service-name churn.
- **Pre-3.24.6 history.** Per-service attribution starts populating after upgrade. Spend accumulated before the upgrade remains in the app total (`current-month-spend-usd`) but is not retroactively attributed.
- **Rolling back.** Total spend survives a downgrade-then-re-upgrade round trip. Per-service attribution does not; an older rack drops the per-service fields, and the breakdown re-populates after re-upgrading.

The breakdown surfaces in `convox cost`, the Console budget panel, and the auto-shutdown `shutdownOrder: largest-cost` ranking. With per-service spends populated, `largest-cost` shuts down the most expensive service first.

## Audit actor

Every budget mutation emits an audit event with an `actor` field identifying who triggered the action. From 3.24.6 onward, when the request supplies an acknowledging identity (the Console passes the authenticated admin's email, and the CLI derives it from your authenticated identity), the rack records that value as the `actor`. When no such value is supplied, the rack falls back to the caller derived from the request credentials (typically `rack-password` for basic-auth clients), preserving pre-3.24.6 behavior.

Operator scripts and webhook receivers ingesting these events should follow the actor-shape guidance in [ack_by Derivation](/migration/ack-by-derivation).

## Troubleshooting

### Breaker re-trips immediately after reset

The cap is below current spend. Either raise the cap or wait for month rollover. `convox budget show myapp` displays current spend versus cap.

### Recovery banner persists across cycles

Pre-3.24.6 racks could carry a dismiss timestamp from one shutdown-and-recovery cycle into the next, silently suppressing the new banner. The fix shipped in 3.24.6. For racks already in a stuck state, clear the annotation manually:

```bash
$ kubectl annotate ns <rack>-<app> convox.com/budget-recovery-banner-dismissed-
```

### Auto-shutdown fired but services did not scale down

Confirm `convox budget show myapp` reports `"at-cap-action": "auto-shutdown"` under `config`. The `atCapAction` key in `convox.yml` does not set it; `convox budget set --at-cap-action` or the Console does. Agent and stateful services are never scaled down. If the countdown armed but did not fire, it may still be running (`notifyBeforeMinutes`), and the shutdown runs on the first accumulator tick at or after the end of the countdown.

### Auto-shutdown fired but I want to keep services running

Run `convox budget reset myapp`. The plain reset restarts services that were scaled to zero. Raising the cap alone (`convox budget cap raise`) does not restart them; if you raise it, run the reset right after. `convox budget reset` is the recovery path after auto-shutdown has fired.

## See Also

- [Cost Tracking](/management/cost-tracking): how spend is computed
- [convox.yml budget block](/configuration/convox-yml#budget): schema reference
- [budget CLI reference](/reference/cli/budget): command reference
- [Webhooks](/configuration/webhooks): receiving cap events at an external URL
- [ack_by Derivation](/migration/ack-by-derivation): actor field semantics for audit-event receivers
- [Budget Management](/console/budget-management): Console UI for budget configuration

> **Note on terminology:** this page covers the **per-app monthly spend cap** introduced in 3.24.6. The unrelated **Karpenter disruption budget** (a cluster-level node-scheduling primitive; see [Karpenter](/configuration/scaling/karpenter)) shares the word "budget" but is a separate concept with no shared configuration surface.
