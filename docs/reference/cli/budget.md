---
title: "budget"
description: "The convox budget command group manages an app's monthly budget cap, its at-cap action, and recovery after a cap is reached."
slug: budget
url: /reference/cli/budget
---
# budget

The `convox budget` command group manages an app's monthly budget cap, its
at-cap action, and recovery after a cap is reached.

For the full operational guide see [Budget Caps](/management/budget-caps); for
the schema reference see the [convox.yml budget block](/configuration/convox-yml#budget).

## budget show

Print the current budget config and runtime state for an app.

### Usage
```bash
    convox budget show <app>
```
### Examples
```bash
    $ convox budget show myapp
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
    convox budget set <app> [--monthly-cap N] [--alert-at N]
                       [--at-cap-action ACTION] [--pricing-adjustment N]
```
### Examples
```bash
    $ convox budget set myapp --monthly-cap 500 --at-cap-action auto-shutdown
    Setting budget for myapp... OK
```

### Prerequisite: cost tracking must be enabled

`budget set` rejects with HTTP 422 when the rack parameter
`cost_tracking_enable` is `false` and you supply any enforcement field
(`--monthly-cap`, `--alert-at`, `--at-cap-action`). Enable cost tracking first:

```bash
$ convox rack params set cost_tracking_enable=true
# wait ~3 min for apply, then:
$ convox budget set myapp --monthly-cap 500 --at-cap-action auto-shutdown
```

Updates that touch only `--pricing-adjustment` are not gated. See
[Cost tracking prerequisite](/management/budget-caps#cost-tracking-prerequisite)
for the full rationale and supported-provider scope. Recovery operations
(`budget clear`, `budget reset`) remain available regardless of cost-tracking
state.

## budget clear

Remove the budget config for an app. The app continues running with no cap,
no threshold, and no auto-shutdown, equivalent to omitting the budget block
from `convox.yml`.

### Usage
```bash
    convox budget clear <app>
```

## budget reset

Acknowledge a cap breach and re-enable deploys. Clears the breaker and, when
run after an auto-shutdown, restarts the services that were scaled down.
Preserves the flap-prevention cooldown by default; `--force-clear-cooldown`
additionally clears the 24-hour cooldown so the next cap fire is not suppressed.

### Usage
```bash
    convox budget reset <app> [--force-clear-cooldown]
```
### Examples
```bash
    $ convox budget reset myapp
    Resetting budget for myapp... OK

    $ convox budget reset myapp --force-clear-cooldown
    Resetting budget for myapp (force-clearing flap-suppress cooldown)... OK
```

See [Force-clear cooldown](/management/budget-caps#force-clear-cooldown) for
when to use the flag.

## budget cap raise

Raise the monthly cap. Atomic with breaker-clear when the new cap is above
current spend. Alias for `budget set --monthly-cap`.

### Usage
```bash
    convox budget cap raise <app> --monthly-cap-usd N
```
### Examples
```bash
    $ convox budget cap raise myapp --monthly-cap-usd 500
    Raising monthly cap for myapp... OK
```

After an auto-shutdown, cap-raise clears the breaker but does NOT restart
already-shutdown services. Run `convox budget reset myapp` to restart them.
See [Cap raise](/management/budget-caps#cap-raise).

## budget simulate-shutdown

Dry-run an auto-shutdown plan without modifying the app. Use it to rehearse
which services would scale down before a real cap fire.

### Usage
```bash
    convox budget simulate-shutdown <app>
```
### Examples
```bash
    $ convox budget simulate-shutdown myapp
    Simulating auto-shutdown for myapp...

    Configuration:
      at_cap_action: auto-shutdown
      webhook URL: https://hooks.example.com/budget
      notify_before_minutes: 10
      shutdown_grace_period: 30s
      shutdown_order: largest-cost-first
      recovery_mode: restore-previous

    Eligibility:
      worker: ELIGIBLE -- replicas=2, cost=$0.40/hr
      api: ELIGIBLE -- replicas=1, cost=$0.20/hr
      web: EXEMPT (neverAutoShutdown)

    Shutdown order (largest-cost-first):
      1. worker -- would scale to 0
      2. api -- would scale to 0

    Estimated savings: $0.60/hr

    Webhook payload sent (dry_run=true):
      See app:budget:auto-shutdown:simulated event in your atCapWebhookUrl webhook delivery and rack log aggregation

    Status: SIMULATION COMPLETE. No changes made.
```

## budget dismiss-recovery

Dismiss the sticky recovery banner that displays after an auto-shutdown
recovers. Equivalent to clicking "Dismiss" in the Console banner.

### Usage
```bash
    convox budget dismiss-recovery <app>
```
### Examples
```bash
    $ convox budget dismiss-recovery myapp
    Banner dismissed for myapp.
```

The dismiss is per-app, not per-user; once dismissed by any user the banner
hides for everyone viewing that app.

## See Also

- [Budget Caps](/management/budget-caps): operational guide
- [Cost Tracking](/management/cost-tracking): how spend is computed
- [convox.yml budget block](/configuration/convox-yml#budget): schema reference
- [cost CLI](/reference/cli/cost): cost breakdown
