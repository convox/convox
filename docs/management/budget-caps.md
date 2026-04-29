---
title: "Budget Caps"
slug: budget-caps
url: /management/budget-caps
---
# Budget Caps

Convox tracks per-app cloud spend and lets you enforce a monthly cap. When the
cap fires, the rack can alert, block new deploys, or auto-shut services down to
prevent overrun. Caps are configured in `convox.yml` (see the [budget block in
convox.yml](/configuration/convox-yml#budget)) and managed at runtime via the
`convox budget` CLI.

This page is the operational guide for managing caps in production. For schema
details see the [convox.yml budget block](/configuration/convox-yml#budget); for
how spend is computed see [Cost Tracking](/management/cost-tracking).

## Prerequisite: cost tracking must be enabled <a id="cost-tracking-prerequisite"></a>

Budget enforcement (`monthlyCapUsd`, `alertThresholdPercent`, `atCapAction`)
requires the rack-level cost accumulator. Without it, no spend is computed —
caps and alerts persist as config but never trip. Enable on AWS racks:

```bash
$ convox rack params set cost_tracking_enable=true
```

If you set a `budget:` block in `convox.yml` (or run `convox budget set`)
against a rack with `cost_tracking_enable=false`, the rack rejects with HTTP
422 and a message pointing at this command. Set the rack parameter, wait for
the apply to complete (~3 min), then redeploy or retry.

`cost_tracking_enable` is AWS-only today; non-AWS racks (Azure, GCP,
DigitalOcean, Equinix Metal, Local) cannot enforce budgets in the current
release. Recovery operations (`convox budget clear`, `convox budget reset`)
remain available regardless of cost tracking state so you can always clean up.

## Cap actions

`atCapAction` selects what happens when an app crosses its `monthlyCapUsd`:

| Action | Behavior |
|--------|----------|
| `alert-only` | Fires `app:budget:cap` and webhooks. No deploy or runtime impact. |
| `block-new-deploys` | Fires `app:budget:cap`, trips the breaker. New deploys are rejected with an over-cap error. Running services keep running. |
| `auto-shutdown` | Arms the auto-shutdown countdown. After `notifyBeforeMinutes`, services are scaled to zero per `shutdownOrder`. |

## Cap raise <a id="cap-raise"></a>

Raising the cap mid-month clears the breaker (when current spend is below the
new cap) and dismisses any active recovery banner.

```bash
$ convox budget cap raise --monthly-cap 500 --app myapp
Raising monthly cap to 500.00 USD... OK
Breaker cleared.
```

`budget cap raise` is an alias for `budget set --monthly-cap`. The clear is
atomic — the rack acquires the per-app lock, writes the new cap, and clears
`CircuitBreakerTripped` + the alert-fired timestamps in the same critical
section. There is no observable window where the new cap is set but the old
breaker is still tripped.

If the new cap is below current spend, the cap-raise rejects with an explicit
error and the breaker remains tripped. Use `convox cost --app myapp` to confirm
current spend before raising.

A cap-raise during the armed window (after `:armed`, before `:fired`) produces
two related events from one operator action, both emitted synchronously from
the same HTTP request handler under the per-app lock: first
`app:budget:breaker-cleared` (with `reason="cap-raised"`) when the cap-raise
atomically clears the tripped deploy circuit breaker, then immediately
`app:budget:auto-shutdown:cancelled` (with `cancel_reason="cap-raised"`)
because the orphan armed shutdown-state annotation is deleted in the same
Namespace Update round-trip. There is no "next accumulator tick" between
them — both events are sent before the cap-raise HTTP request returns.
Receivers correlating the audit pair should match on:
1. `data.app` — identical on both events.
2. The operator identity — `data.ack_by` on `:breaker-cleared` and
   `data.actor` on `:cancelled` (different field names, same JWT-derived
   value sourced from the cap-raiser).
3. `data.cleared_at` on `:breaker-cleared` and `data.cancelled_at` on
   `:cancelled` — bit-identical RFC 3339 timestamps, both populated from
   the single `breakerClearedAckAt` value captured at the breaker-clear
   site (not two `time.Now()` calls), so receivers can match on exact
   string equality.
Receivers cannot use `tick_id` to correlate the pair: only the auto-shutdown
lifecycle events carry `tick_id` in their universal payload, and
`:breaker-cleared` is a top-level audit event whose payload does not include
that field. The two events are intentional and not duplicates —
`:breaker-cleared` covers the deploy-circuit-breaker side effect (deploys are
unblocked), while `:cancelled reason=cap-raised` records that the
auto-shutdown countdown was aborted.

After `:fired` (post-shutdown), a cap-raise alone clears the breaker but
does NOT restart already-shutdown services on its own — the cap-raise is
limited to the cap value and breaker. Run `convox budget reset myapp` to
clear the breaker AND restore replicas from the persisted shutdown-state
annotation (`restoreFromAnnotation`). See
[Reset and force-clear cooldown](#force-clear-cooldown) below.

## Reset and force-clear cooldown <a id="force-clear-cooldown"></a>

`convox budget reset` acknowledges a cap breach and re-enables deploys. The
plain reset clears the deploy circuit breaker AND, when invoked after `:fired`,
restores replicas from the persisted shutdown-state annotation
(`restoreFromAnnotation`). The default behavior preserves any flap-suppress
carry-over so that an app that recently breached, was reset, then breached
again does not flip-flop into auto-shutdown loops.

```bash
$ convox budget reset --app myapp
Resetting budget for myapp... OK
Breaker cleared.
```

`--force-clear-cooldown` is additive — it does not change the breaker-clear
or the replica-restore behavior, but it additionally clears the flap-suppress
annotation so the next cap fire will not be suppressed by the 24-hour
flap-prevention cooldown. Use only when you are sure the underlying cause is
resolved.

```bash
$ convox budget reset --app myapp --force-clear-cooldown
Resetting budget for myapp (force-clearing flap-suppress cooldown)... OK
```

## Block-new-deploys recovery <a id="block-new-deploys"></a>

When `atCapAction: block-new-deploys` fires, deploys are rejected with:

```
budget cap exceeded: monthly cap 250.00 USD, current spend 268.42 USD
```

To recover:

1. Run `convox cost --app myapp` to confirm current spend.
2. Either raise the cap (`convox budget cap raise --monthly-cap NEW`) or wait
   for the next month rollover (current spend resets on the 1st).
3. Or accept the cap and reset to re-enable deploys for the rest of the month
   without raising — `convox budget reset`. The breaker clears but the cap
   remains; subsequent cost growth will trip the breaker again.

## Per-service cost breakdown <a id="per-service-breakdown"></a>

`convox cost --app myapp` returns a `breakdown` array with the cumulative
spend, instance type, and bucket name for every service that has been
observed running this month. The breakdown populates from accumulator ticks
(default 10 minutes apart) and grows monotonically until month rollover, at
which point it resets to zero alongside `currentMonthSpendUsd`.

Two reserved buckets surface alongside service names:

- `_build` — build pods carry `service-type=build` plus a `service` label
  naming the service being built. Their spend is bucketed away from that
  service so the named service's normal-operation cost stays uninflated.
- `_unattributed` — pods with no `service` label (system-injected sidecars,
  KEDA scalers, anything not user-deployed). Their spend stays visible in
  the breakdown without polluting any user-deployed service's row.

Edge cases:

- **Service deleted mid-month.** The deleted service's accumulated spend
  remains in the breakdown until the month rollover. Operator intuition: "I
  ran this service for a week, it cost $50; deleting it doesn't make the $50
  go away."
- **Service renamed mid-month.** The old name keeps its pre-rename spend;
  the new name accumulates from the rename point forward. Both rows appear
  until rollover, summing to the correct app total.
- **Bucket cap.** The breakdown is capped at 1000 entries per app. Once full,
  existing rows continue to accumulate but new services are dropped from this
  month's breakdown. The rack logs `at=per_service_truncated count=N` and
  also fires an `app:budget:per-service-truncated` audit event on every
  truncating tick (`data.dropped`, `data.cap`, `data.app`); subscribe via
  webhook or `convox events list -a <app>` to surface it without log
  access. Practical apps stay well under the cap; if you hit it, audit the
  per-tick label set for unbounded service-name churn.
- **Pre-3.24.6rc5 history.** Per-service attribution starts populating from
  the first tick after upgrade. Spend that accumulated before the upgrade
  remains in `currentMonthSpendUsd` (the total) but is not retroactively
  attributed.
- **Rolling back from rc5.** Total spend (`currentMonthSpendUsd`) survives a
  downgrade-then-re-upgrade round-trip. Per-service attribution does not —
  the older binary drops the unknown fields on its first tick. After
  re-upgrading, the breakdown re-populates from the next tick forward.

The breakdown surfaces in `convox cost`, the Console budget panel, and the
auto-shutdown `shutdownOrder: largest-cost` ranking. With per-service spends
populated, `largest-cost` shuts down the most expensive service first.

## Audit actor resolution <a id="audit-actor"></a>

Every budget mutation emits an audit event with an `actor` field identifying
who triggered the action. From 3.24.6 onward the rack honors a
`ack_by` form parameter passed alongside the request body and records its
value as the persisted `actor` instead of the basic-auth literal
`rack-password`. When the form parameter is absent or empty, the rack falls
back to the JWT-derived caller (typically `rack-password` for basic-auth
clients or the system user for internal callers), preserving pre-3.24.6
behavior.

The four mutation handlers wired through this resolution are:

| Handler | Event | Where the resolved actor lands |
|---|---|---|
| `AppBudgetSet` | `app:budget:set` | `cfg.LastCapMutationBy` + `data.actor` |
| `AppBudgetClear` | `app:budget:clear` | `data.actor` (also `prev_ack_by` metadata) |
| `AppBudgetReset` | `app:budget:reset` | `state.CircuitBreakerAckBy` + `data.actor` |
| `AppBudgetDismissRecovery` | `app:budget:dismiss-recovery` | `data.actor` |

Console (3.24.6+) populates the form parameter with the authenticated
admin's email so the rack records `actor=alice@example.com` rather than
the generic `rack-password` sentinel. When the form parameter is honored,
the rack's HTTP response carries an RFC 8594 deprecation triple:

```
Deprecation: true
Sunset: Thu, 01 Oct 2026 00:00:00 GMT
Link: <https://docs.convox.com/migration/ack-by-derivation>; rel="deprecation"; type="text/html"
```

The `Sunset` date is a courtesy hint per [RFC 8594](https://www.rfc-editor.org/rfc/rfc8594) —
it signals migration intent, not a binding deadline. The rack will continue
to accept the form parameter beyond that date and may extend or remove the
header in any future release. Migration to per-user JWT Bearer authentication
is targeted for a 3.25.0+ release; until that ships, the form parameter is
both the bridge and the migration target. Operator scripts and webhook
receivers ingesting these events should follow the actor-shape guidance in
[ack_by Derivation](/migration/ack-by-derivation).

### Backward compatibility

Pre-3.24.6 racks (3.24.5 and earlier) silently ignore the deprecation-signal
layer — they predate the `Sunset`/`Deprecation`/`Link` triple — but still
record the form-parameter value into audit events. Mixed-version deployments
are safe in both directions:

- **3.24.6 Console + 3.24.5 rack** — actor still records the authenticated
  email; no rack-emitted deprecation triple. Console self-mirrors the same
  triple onto its own GraphQL response so the GUI sunset banner still
  renders.
- **3.24.5 Console + 3.24.6 rack** — actor records the older Console's
  literal fallback (`console`); rack still emits the deprecation triple,
  but the older Vue layer has no banner widget and silently drops the
  headers.
- **Older Console / no form parameter** — rack falls back to the JWT-derived
  caller with pre-3.24.6 audit behavior; no deprecation triple is emitted.

The deprecation triple is gated on a non-empty form parameter; an empty or
absent parameter never produces deprecation headers.

## Troubleshooting <a id="troubleshooting"></a>

### Breaker re-trips immediately after reset

The cap is below current spend. Either raise the cap or wait for month rollover.
`convox budget show --app myapp` displays current spend vs cap.

### Recovery banner persists across cycles

Pre-3.24.6 racks had a leak where the dismiss timestamp could carry from one
ARMED→RECOVERED cycle into the next, silently suppressing the new banner. The
fix landed in 3.24.6's `runStaleAnnotationGC`. For racks already in stuck state,
clear the annotation manually:

```bash
$ kubectl annotate ns <rack>-<app> convox.com/budget-recovery-banner-dismissed-
```

### Auto-shutdown fired but services did not scale down

Confirm `convox.yml` has `atCapAction: auto-shutdown` set (not `block-new-deploys`).
Check `convox budget show --app myapp` for the live state. If `:armed` fired but
not `:fired`, the countdown may still be running (`notifyBeforeMinutes`).

### `:fired` fired but I want to keep services running

Run `convox budget reset myapp`. The plain reset clears the breaker AND
restarts shutdown services from the persisted shutdown-state annotation.
A cap-raise alone (`convox budget cap raise`) clears the breaker but does
not restart shutdown services — `budget reset` is the canonical recovery
path post-`:fired`.

## See Also

- [Cost Tracking](/management/cost-tracking) — how spend is computed
- [convox.yml budget block](/configuration/convox-yml#budget) — schema reference
- [budget CLI reference](/reference/cli/budget) — command reference
- [Webhooks](/configuration/webhooks) — receiving cap events at an external URL
- [ack_by Derivation](/migration/ack-by-derivation) — actor field semantics for audit-event receivers

> **Note on terminology:** this page covers the **per-app monthly spend cap** introduced in 3.24.6. The unrelated **Karpenter disruption budget** (cluster-level node-scheduling primitive — see [Karpenter](/configuration/scaling/karpenter)) shares the word "budget" but is a separate concept with no shared configuration surface.
