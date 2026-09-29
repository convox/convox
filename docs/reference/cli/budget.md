---
title: "budget"
description: "The convox budget command group manages an app's monthly budget cap, its at-cap action, and recovery after a cap is reached."
slug: budget
url: /reference/cli/budget
---
# budget

The `convox budget` command group manages an app's monthly budget cap, its at-cap action, and recovery after a cap is reached.

For the full operational guide see [Budget Caps](/management/budget-caps); for the schema reference see the [convox.yml budget block](/configuration/convox-yml#budget).

## budget show

Print the current budget config and runtime state for an app as JSON. When the app has no budget config, the command prints `no budget configured for <app>`.

### Usage
```bash
    convox budget show <app>
```
### Examples
```bash
    $ convox budget show myapp
    {
      "config": {
        "monthly-cap-usd": 250,
        "alert-threshold-percent": 80,
        "at-cap-action": "block-new-deploys",
        "pricing-adjustment": 1,
        "last-cap-mutation-by": "alice"
      },
      "state": {
        "month-start": "2026-09-01T00:00:00Z",
        "current-month-spend-usd": 134.65,
        "current-month-spend-as-of": "2026-09-29T13:40:00Z",
        "alert-fired-at-threshold": "0001-01-01T00:00:00Z",
        "alert-fired-at-cap": "0001-01-01T00:00:00Z",
        "circuit-breaker-ack-at": "0001-01-01T00:00:00Z",
        "per-service-spend-usd": {
          "web": 134.65
        },
        "per-service-instance-type": {
          "web": "t3.large"
        },
        "per-service-spend-by-variant": {
          "web": {
            "t3.large:on-demand": 134.65
          }
        },
        "per-service-variant-pods-last-tick": {
          "web": {
            "t3.large:on-demand": 2
          }
        }
      }
    }
```

A timestamp that has not been set prints as `0001-01-01T00:00:00Z`. `circuit-breaker-tripped` appears as `true` while `block-new-deploys` is blocking deploys, and `warning-count` appears when the last tick could not price every pod. While the app has an auto-shutdown in progress, a one-line `[ARMED]`, `[ACTIVE]`, `[FAILED]` or `[RECOVERED]` banner precedes the JSON. While deploys are blocked and a service uses KEDA, a notice that KEDA-managed services may still scale precedes it too.

## budget set

Set or update the app's budget config: the monthly cap, alert threshold, at-cap action and pricing adjustment. These values come only from this command, `convox budget cap raise` and the Console. The same keys in a `convox.yml` `budget:` block do not set them, and a deploy does not change them.

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

`budget set` rejects with HTTP 422 when the rack parameter `cost_tracking_enable` is `false` and you supply any enforcement field (`--monthly-cap`, `--alert-at`, `--at-cap-action`). Enable cost tracking first:

```bash
$ convox rack params set cost_tracking_enable=true
# wait for the Rack update to finish, then:
$ convox budget set myapp --monthly-cap 500 --at-cap-action auto-shutdown
```

Updates that touch only `--pricing-adjustment` are not gated. See [Cost tracking prerequisite](/management/budget-caps#prerequisite-cost-tracking-must-be-enabled) for the full rationale and supported-provider scope. Recovery operations (`budget clear`, `budget reset`) remain available regardless of cost-tracking state.

## budget clear

Remove the budget config for an app. The app continues running with no cap, no threshold, and no auto-shutdown. Clearing also deletes the app's stored spend, so month-to-date spend and the per-service breakdown start again from zero on the next accumulator tick. Requires the Admin role.

### Usage
```bash
    convox budget clear <app>
```
### Examples
```bash
    $ convox budget clear myapp
    Clearing budget for myapp... OK
```

## budget reset

Acknowledge a cap breach and re-enable deploys. Clears the breaker and, when run after an auto-shutdown, restarts the services that were scaled down. Preserves the flap-prevention cooldown by default; `--force-clear-cooldown` additionally clears the 24-hour cooldown so the next cap fire is not suppressed.

The command asks for confirmation first and refuses to prompt when stdin is not a terminal. Pass `-f` or `--force` to skip the prompt, for example in a script.

### Usage
```bash
    convox budget reset <app> [--force] [--force-clear-cooldown]
```
### Examples
```bash
    $ convox budget reset myapp
    This will acknowledge the current spend and re-enable deploys for myapp. Continue? [y/N]: y
    Resetting budget for myapp... OK

    $ convox budget reset myapp --force-clear-cooldown --force
    Resetting budget for myapp... OK
```

If spend is still at or above the cap, the next accumulator tick, within 10 minutes, fires the cap again. See [Force-clear cooldown](/management/budget-caps#reset-and-force-clear-cooldown) for when to use the flag.

## budget cap raise

Raise the monthly cap. Atomic with breaker-clear when the new cap is above current spend. Alias for `budget set --monthly-cap`.

### Usage
```bash
    convox budget cap raise <app> --monthly-cap-usd N
```
### Examples
```bash
    $ convox budget cap raise myapp --monthly-cap-usd 500
    Raising monthly cap for myapp... OK
```

If the new cap is below current spend, the CLI prints a warning that the cap will trip on the next accumulator tick, and the Rack saves the cap anyway.

Raising the cap does not restart services that auto-shutdown scaled to zero. Run `convox budget reset myapp` right after the raise to restore them. See [Cap raise](/management/budget-caps#raising-or-recovering-a-cap).

## budget simulate-shutdown

Dry-run an auto-shutdown plan without modifying the app. Use it to rehearse which services would scale down before a real cap fire. The dry run sends an `app:budget:auto-shutdown:simulated` event to the Rack's webhooks. Each service's `cost` is its month-to-date spend divided by the hours since the month started, not its current rate. Agent and stateful services are always exempt.

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
      shutdown_order: largest-cost
      recovery_mode: auto-on-reset

    Eligibility:
      worker: ELIGIBLE -- replicas=2, cost=$0.40/hr
      api: ELIGIBLE -- replicas=1, cost=$0.20/hr
      web: EXEMPT (in neverAutoShutdown)
      metrics: EXEMPT (agent service (DaemonSet))

    Shutdown order (largest-cost):
      1. worker -- would scale to 0
      2. api -- would scale to 0

    Estimated savings: $0.60/hr

    Webhook payload sent (dry_run=true):
      See app:budget:auto-shutdown:simulated event in your atCapWebhookUrl webhook delivery and rack log aggregation

    Status: SIMULATION COMPLETE. No changes made.
```

## budget dismiss-recovery

Dismiss the sticky recovery banner that displays after an auto-shutdown recovers. Equivalent to clicking "Dismiss" in the Console banner.

### Usage
```bash
    convox budget dismiss-recovery <app>
```
### Examples
```bash
    $ convox budget dismiss-recovery myapp
    Banner dismissed for myapp.
```

The dismiss is per-app, not per-user; once dismissed by any user the banner hides for everyone viewing that app.

## See Also

- [Budget Caps](/management/budget-caps): operational guide
- [Cost Tracking](/management/cost-tracking): how spend is computed
- [convox.yml budget block](/configuration/convox-yml#budget): schema reference
- [cost CLI](/reference/cli/cost): cost breakdown
