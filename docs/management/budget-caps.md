---
title: "Budget Caps"
description: "Budget Caps track per-app cloud spend and enforce a monthly cap that can alert, block new deploys, or auto-shutdown services when reached."
slug: budget-caps
url: /management/budget-caps
---
# Budget Caps

Convox tracks per-app cloud spend and lets you enforce a monthly cap. When the
cap is reached, the rack can alert, block new deploys, or automatically shut
services down to prevent overrun. Caps are managed at runtime with the
`convox budget` CLI or the Console budget tab. The
[budget block in convox.yml](/configuration/convox-yml#budget) declares the
schema and is validated on `convox releases promote`, but it does not itself
persist runtime cap values. Set caps with `convox budget set` so cap changes
are explicit, audit-attributed, and not implicitly overwritten by the next
deploy.

This page is the operational guide for managing caps in production. For schema
details see the [convox.yml budget block](/configuration/convox-yml#budget); for
how spend is computed see [Cost Tracking](/management/cost-tracking).

## Prerequisite: cost tracking must be enabled <a id="cost-tracking-prerequisite"></a>

Budget enforcement (`monthlyCapUsd`, `alertThresholdPercent`, `atCapAction`)
requires the rack-level cost accumulator. Without it, no spend is computed, so
caps and alerts persist as config but never trip. Enable it on AWS racks:

```bash
$ convox rack params set cost_tracking_enable=true
```

If you run `convox budget set` (or deploy a manifest with a `budget:` block)
against a rack with `cost_tracking_enable=false`, the rack rejects the request
with HTTP 422 and a message pointing at this command. Set the rack parameter,
wait for the apply to complete (~3 min), then redeploy or retry.

`cost_tracking_enable` is AWS-only today; non-AWS racks (Azure, GCP,
DigitalOcean, Equinix Metal, Local) cannot enforce budgets in the current
release. Recovery operations (`convox budget clear`, `convox budget reset`)
remain available regardless of cost tracking state so you can always clean up.

## Set a monthly cap <a id="set-a-cap"></a>

Set a cap for an app with `convox budget set`:

```bash
$ convox budget set myapp --monthly-cap 500
```

The cap value is in USD. You can configure the alert threshold, the at-cap
action, and a pricing adjustment in the same command:

```bash
$ convox budget set myapp --monthly-cap 1000 --alert-at 75 --at-cap-action block-new-deploys --pricing-adjustment 0.7
```

| Flag | Default | Effect |
|---|---|---|
| `--monthly-cap` | none | Monthly spend cap in USD. Setting it enables enforcement for the app. |
| `--alert-at` | `80` | Percent of the cap at which an alert fires (the threshold alert). |
| `--at-cap-action` | `alert-only` | What happens when spend crosses the cap. See [Cap actions](#cap-actions). |
| `--pricing-adjustment` | `1.0` | Multiplier applied to computed spend (e.g. `0.7` to model a 30% committed-use discount). |

You can also set these in the [convox.yml budget block](/configuration/convox-yml#budget),
but runtime cap values come from `convox budget set`, not from the manifest.

## Cap actions <a id="cap-actions"></a>

`atCapAction` selects what happens when an app crosses its `monthlyCapUsd`:

| Action | Behavior |
|--------|----------|
| `alert-only` (default) | Fires the `app:budget:cap` event and webhooks. No deploy or runtime impact. |
| `block-new-deploys` | Fires `app:budget:cap` and blocks new deploys with an over-cap error. Running services keep running. |
| `auto-shutdown` | Starts the auto-shutdown countdown. After `notifyBeforeMinutes`, services are scaled to zero in `shutdownOrder` order. |

When you choose `auto-shutdown`, the CLI prints a warning reminding you to
configure your at-cap webhook and to validate the configuration first:

```bash
$ convox budget set myapp --monthly-cap 500 --at-cap-action auto-shutdown
```

Run `convox budget simulate-shutdown myapp` to preview which services would be
scaled to zero, and in what order, without actually shutting anything down.

### Choosing an action

- Use `alert-only` while you are still learning what an app costs. You get the
  threshold alert and the at-cap event without any runtime impact.
- Use `block-new-deploys` to stop new deploys from adding more cost without
  touching what is already running. Good for non-production apps that should
  not grow further this month.
- Use `auto-shutdown` for apps where stopping the spend matters more than
  staying up. Pair it with a configured webhook so your team is paged on the
  `:armed` event before services scale down.

## Raising or recovering a cap <a id="cap-raise"></a>

Raising the cap mid-month re-enables blocked deploys (when current spend is
below the new cap) and dismisses any active recovery banner:

```bash
$ convox budget cap raise myapp --monthly-cap-usd 500
Raising monthly cap for myapp... OK
```

`budget cap raise` is an alias for `budget set --monthly-cap`. Both
`--monthly-cap-usd` and `--monthly-cap` are accepted.

If the new cap is below current spend, the cap-raise is rejected with an
explicit error and deploys stay blocked. Use `convox cost --app myapp` to
confirm current spend before raising.

After auto-shutdown has fired, a cap-raise alone re-enables deploys but does
**not** restart already-shutdown services. To clear the breach and restore
services, use `convox budget reset` (see below).

## Reset and force-clear cooldown <a id="force-clear-cooldown"></a>

`convox budget reset` acknowledges a cap breach and re-enables deploys. After
auto-shutdown has fired, the plain reset also restarts services that were
scaled to zero. The cap value itself is unchanged.

```bash
$ convox budget reset myapp
Resetting budget for myapp... OK
```

By default, reset preserves the flap-suppression cooldown: an app that recently
breached, was reset, then breached again will not flip-flop into repeated
auto-shutdown cycles within 24 hours.

`--force-clear-cooldown` is additive. It re-enables deploys and restores
services exactly as the plain reset does, and additionally clears the
flap-suppression cooldown so the next cap breach is not suppressed by the
24-hour window. Use it only when you are sure the underlying cause is resolved.

```bash
$ convox budget reset myapp --force-clear-cooldown
Resetting budget for myapp (force-clearing flap-suppress cooldown)... OK
```

## Block-new-deploys recovery <a id="block-new-deploys"></a>

When `atCapAction: block-new-deploys` is in effect and the cap is reached,
deploys are rejected with an over-cap error similar to:

```text
budget cap exceeded for app myapp: spent $268.42 of $250.00 cap this month
```

To recover:

1. Run `convox cost --app myapp` to confirm current spend.
2. Either raise the cap (`convox budget cap raise --monthly-cap-usd NEW`) or wait
   for the next month rollover (current spend resets on the 1st).
3. Or accept the cap and run `convox budget reset` to re-enable deploys for the
   rest of the month without raising the cap. The breaker clears but the cap
   remains; subsequent cost growth will block deploys again.

## What you see when a cap is hit <a id="sub-state-vocabulary"></a>

The `convox ps` `STATUS` column and the `convox services` `BUDGET` column both
show a per-service sub-state token when an app's budget cap is breached or
arming. The vocabulary is the same on both surfaces:

| Token | Meaning |
|-------|---------|
| `armed-Nm` | Auto-shutdown countdown is active; `N` minutes remain until services scale to zero (e.g. `armed-25m`). |
| `at-cap-keda` | Service has been scaled to zero by auto-shutdown (KEDA-managed services). |
| `at-cap-auto` | Service has been scaled to zero by auto-shutdown (deployment-only services). |
| `at-cap` | Cap is breached with `atCapAction: block-new-deploys`; no scale-to-zero, deploys are rejected. |

When you recover (raise the cap or reset), these tokens clear and any recovery
banner in the Console is dismissed.

## Authorization <a id="authorization"></a>

Budget operations split across two authorization tiers. Cap mutation, clearing
budget config, and force-clearing the cooldown require the Admin role.
Threshold-only changes, plain reset, dismiss-recovery, and the read-only
simulate-shutdown preview require the read-write (`rw`) role. The split applies
on every provider (AWS, Azure, GCP, DigitalOcean, Equinix Metal, Local).

| Operation | Required role |
|---|---|
| Set or raise the monthly cap (`--monthly-cap`) | Admin |
| Set the at-cap action (`--at-cap-action`) | Admin |
| Set the pricing adjustment (`--pricing-adjustment`) | Admin |
| Set the alert threshold only (`--alert-at`) | `rw` |
| Clear budget config (`convox budget clear`) | Admin |
| Reset (`convox budget reset`) | `rw` |
| Reset with `--force-clear-cooldown` | Admin |
| Dismiss recovery banner (`convox budget dismiss-recovery`) | `rw` |
| Simulate shutdown (`convox budget simulate-shutdown`, read-only) | `rw` |

The Admin requirement on cap changes, clear, and force-clear keeps an admin-set
cap from being circumvented by a non-admin (for example, clearing the budget
and re-setting it without the cap).

Basic-auth callers (using the rack password) pass the Admin check
automatically. A non-Admin Console user who attempts an Admin-only action sees
the rejection message relayed through the Console UI. The exact rejection
messages are:

```text
403 AppBudgetSet: admin role required to set budget cap
403 AppBudgetClear: admin role required to remove budget config
403 AppBudgetReset --force-clear-cooldown requires Admin role; current role is 'w'. Contact rack admin or use Admin token.
```

## Per-service cost breakdown <a id="per-service-breakdown"></a>

`convox cost --app myapp` returns a `breakdown` array with the cumulative
spend, instance type, and bucket name for every service that has been observed
running this month. The breakdown grows over the month and resets to zero at
month rollover alongside `currentMonthSpendUsd`.

Two reserved buckets appear alongside service names:

- `_build`: spend for build pods, bucketed away from the service being built so
  the named service's normal-operation cost stays uninflated.
- `_unattributed`: spend for pods with no service label (system sidecars,
  autoscaler components, anything not user-deployed). Kept visible without
  inflating any user-deployed service's row.

Edge cases:

- **Service deleted mid-month.** The deleted service's accumulated spend
  remains in the breakdown until month rollover. You ran the service for part
  of the month, so its cost stays attributed for that month.
- **Service renamed mid-month.** The old name keeps its pre-rename spend; the
  new name accumulates from the rename point forward. Both rows appear until
  rollover and sum to the correct app total.
- **Per-app entry cap.** The breakdown holds up to 1000 services per app. Once
  full, existing rows keep accumulating but new services are dropped from this
  month's breakdown, and an `app:budget:per-service-truncated` event fires.
  Subscribe via webhook to surface it. Most
  apps stay well under the cap; if you hit it, check for unbounded
  service-name churn.
- **Pre-3.24.6 history.** Per-service attribution starts populating after
  upgrade. Spend accumulated before the upgrade remains in the app total
  (`currentMonthSpendUsd`) but is not retroactively attributed.
- **Rolling back.** Total spend survives a downgrade-then-re-upgrade round
  trip. Per-service attribution does not; an older rack drops the per-service
  fields, and the breakdown re-populates after re-upgrading.

The breakdown surfaces in `convox cost`, the Console budget panel, and the
auto-shutdown `shutdownOrder: largest-cost` ranking. With per-service spends
populated, `largest-cost` shuts down the most expensive service first.

## Audit actor <a id="audit-actor"></a>

Every budget mutation emits an audit event with an `actor` field identifying
who triggered the action. From 3.24.6 onward, when the request supplies an
acknowledging identity (the Console passes the authenticated admin's email, and
the CLI derives it from your authenticated identity), the rack records that value as the
`actor`. When no such value is supplied, the rack falls back to the caller
derived from the request credentials (typically `rack-password` for basic-auth
clients), preserving pre-3.24.6 behavior.

Operator scripts and webhook receivers ingesting these events should follow the
actor-shape guidance in [ack_by Derivation](/migration/ack-by-derivation).

## Troubleshooting <a id="troubleshooting"></a>

### Breaker re-trips immediately after reset

The cap is below current spend. Either raise the cap or wait for month
rollover. `convox budget show myapp` displays current spend versus cap.

### Recovery banner persists across cycles

Pre-3.24.6 racks could carry a dismiss timestamp from one shutdown-and-recovery
cycle into the next, silently suppressing the new banner. The fix shipped in
3.24.6. For racks already in a stuck state, clear the annotation manually:

```bash
$ kubectl annotate ns <rack>-<app> convox.com/budget-recovery-banner-dismissed-
```

### Auto-shutdown fired but services did not scale down

Confirm `convox.yml` has `atCapAction: auto-shutdown` set (not
`block-new-deploys`). Check `convox budget show myapp` for the live state. If
the countdown armed but did not fire, it may still be running
(`notifyBeforeMinutes`).

### Auto-shutdown fired but I want to keep services running

Run `convox budget reset myapp`. The plain reset re-enables deploys AND
restarts services that were scaled to zero. A cap-raise alone
(`convox budget cap raise`) re-enables deploys but does not restart services.
`convox budget reset` is the recovery path after auto-shutdown has fired.

## See Also

- [Cost Tracking](/management/cost-tracking): how spend is computed
- [convox.yml budget block](/configuration/convox-yml#budget): schema reference
- [budget CLI reference](/reference/cli/budget): command reference
- [Webhooks](/configuration/webhooks): receiving cap events at an external URL
- [ack_by Derivation](/migration/ack-by-derivation): actor field semantics for audit-event receivers
- [Budget Management](/console/budget-management): Console UI for budget configuration

> **Note on terminology:** this page covers the **per-app monthly spend cap** introduced in 3.24.6. The unrelated **Karpenter disruption budget** (a cluster-level node-scheduling primitive; see [Karpenter](/configuration/scaling/karpenter)) shares the word "budget" but is a separate concept with no shared configuration surface.
