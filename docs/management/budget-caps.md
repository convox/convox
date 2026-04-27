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

After `:fired` (post-shutdown), a cap raise clears the breaker but does NOT
restart already-shutdown services on its own. Run `convox budget reset myapp`
to restore replicas from the persisted shutdown-state annotation
(`restoreFromAnnotation`). See the [3.24.6 release notes](https://github.com/convox/convox/releases)
for the full sequence.

## Reset and force-clear cooldown <a id="force-clear-cooldown"></a>

`convox budget reset` acknowledges a cap breach and re-enables deploys. The
default behavior preserves any flap-suppress carry-over so that an app that
recently breached, was reset, then breached again does not flip-flop into
auto-shutdown loops.

```bash
$ convox budget reset --app myapp
Resetting budget for myapp... OK
Breaker cleared.
```

`--force-clear-cooldown` additionally clears the flap-suppress annotation so
the next cap fire will not be suppressed. Use only when you are sure the
underlying cause is resolved.

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

Cap-raise post-`:fired` clears the breaker but does not restart shutdown
services. Run `convox budget reset myapp` to restore replicas from the
persisted shutdown-state annotation.

## See Also

- [Cost Tracking](/management/cost-tracking) — how spend is computed
- [convox.yml budget block](/configuration/convox-yml#budget) — schema reference
- [budget CLI reference](/reference/cli/budget) — command reference
- [Webhooks](/configuration/webhooks) — receiving cap events at an external URL

> **Note on terminology:** this page covers the **per-app monthly spend cap** introduced in 3.24.6. The unrelated **Karpenter disruption budget** (cluster-level node-scheduling primitive — see [Karpenter](/configuration/scaling/karpenter)) shares the word "budget" but is a separate concept with no shared configuration surface.
