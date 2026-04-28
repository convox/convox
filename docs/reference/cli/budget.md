---
title: "budget"
slug: budget
url: /reference/cli/budget
---
# budget

The `convox budget` command group manages an app's monthly budget cap, the
`atCapAction` enforcement, and recovery from a cap fire.

For the operational guide see [Budget Caps](/management/budget-caps); for the
schema reference see the [convox.yml budget block](/configuration/convox-yml#budget).

## budget show

Print the current budget config and runtime state for an app.

### Usage
```bash
    convox budget show [-a app]
```
### Examples
```bash
    $ convox budget show --app myapp
    Monthly cap     250.00 USD
    Spend           134.65 USD (53.86%)
    At-cap action   auto-shutdown
    Breaker         clear
    State           idle
```

## budget set

Set or update the budget config in `convox.yml`-equivalent shape. Equivalent
to editing the manifest and redeploying, but applied without a redeploy.

### Usage
```bash
    convox budget set [-a app] [--monthly-cap N] [--alert-at N]
                       [--at-cap-action ACTION] [--pricing-adjustment N]
```
### Examples
```bash
    $ convox budget set --monthly-cap 500 --at-cap-action auto-shutdown --app myapp
    Setting budget for myapp... OK
```

## budget clear

Remove the budget config for an app. The app continues running with no cap,
no threshold, and no auto-shutdown — equivalent to omitting the budget block
from `convox.yml`.

### Usage
```bash
    convox budget clear [-a app]
```

## budget reset

Acknowledge a cap breach and re-enable deploys. Clears the breaker AND, when
invoked after `:fired`, restores replicas from the persisted shutdown-state
annotation. Preserves flap-suppress carry-over by default;
`--force-clear-cooldown` is additive and forces past the 24-hour
flap-prevention cooldown.

### Usage
```bash
    convox budget reset [-a app] [--force-clear-cooldown]
```
### Examples
```bash
    $ convox budget reset --app myapp
    Resetting budget for myapp... OK
    Breaker cleared.

    $ convox budget reset --app myapp --force-clear-cooldown
    Resetting budget for myapp (force-clearing flap-suppress cooldown)... OK
```

See [Force-clear cooldown](/management/budget-caps#force-clear-cooldown) for
when to use the flag.

## budget cap raise

Raise the monthly cap. Atomic with breaker-clear when the new cap is above
current spend. Alias for `budget set --monthly-cap`.

### Usage
```bash
    convox budget cap raise [-a app] --monthly-cap N
```
### Examples
```bash
    $ convox budget cap raise --app myapp --monthly-cap 500
    Raising monthly cap to 500.00 USD... OK
    Breaker cleared.
```

After `:fired` (post-shutdown), cap-raise clears the breaker but does NOT
restart already-shutdown services. Run `convox budget reset myapp` to
restore replicas from the persisted shutdown-state annotation. See
[Cap raise](/management/budget-caps#cap-raise).

## budget simulate-shutdown

Dry-run an auto-shutdown plan without modifying the app. Emits the
`:simulated` audit event so operators can rehearse the failure path.

### Usage
```bash
    convox budget simulate-shutdown [-a app]
```
### Examples
```bash
    $ convox budget simulate-shutdown --app myapp
    Simulating auto-shutdown for myapp...
    Plan (largest-cost order):
      worker (eligible)
      api (eligible)
      web (excluded — neverAutoShutdown)
    OK
```

## budget dismiss-recovery

Dismiss the sticky recovery banner that displays after an auto-shutdown
recovers. Equivalent to clicking "Dismiss" in the Console banner.

### Usage
```bash
    convox budget dismiss-recovery [-a app]
```
### Examples
```bash
    $ convox budget dismiss-recovery --app myapp
    Dismissing recovery banner for myapp... OK
```

The dismiss is per-app, not per-user; once dismissed by any user the banner
hides for everyone viewing that app. See [Webhook Signing](/console/webhook-signing#receiver-migration)
for the audit event semantics.

## See Also

- [Budget Caps](/management/budget-caps) — operational guide
- [Cost Tracking](/management/cost-tracking) — how spend is computed
- [convox.yml budget block](/configuration/convox-yml#budget) — schema reference
- [cost CLI](/reference/cli/cost) — cost breakdown
