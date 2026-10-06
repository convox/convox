---
title: "budget cap"
description: "The convox budget cap command group operates on the monthlyCapUsd field of an app's budget config and currently exposes the raise subcommand."
slug: budget-cap
url: /reference/cli/budget-cap
---
# budget cap

The `budget cap` command group operates on the `monthlyCapUsd` field of an app's budget config. Currently exposes a single subcommand: `raise`. Lowering or removing the cap is done through `convox budget set` or `convox budget clear`.

## budget cap raise

Raise the monthly cap. When the new cap is above both the previous cap and current spend, the same operation clears the breaker and, on Rack version `3.25.10` or later, resets the threshold and cap alerts and cancels an armed auto-shutdown countdown.

> **Requires admin role on the rack.** A non-admin caller (`rw` role) receives `403 AppBudgetSet: admin role required to set budget cap`. Basic-auth (rack-password) callers automatically pass the admin check.

### Usage
```bash
    convox budget cap raise <app> --monthly-cap-usd N
```

### Examples
```bash
    $ convox budget cap raise myapp --monthly-cap-usd 500
    Raising monthly cap for myapp... OK
```

If the new cap is below current spend, the CLI prints a warning and the Rack saves the cap anyway, so it trips again on the next accumulator tick:

```bash
    $ convox budget cap raise myapp --monthly-cap-usd 100
    WARNING: --monthly-cap-usd=$100.00 is below current month-to-date spend $134.65. Cap will trip immediately on next accumulator tick.
    Raising monthly cap for myapp... OK
```

Use `convox cost --app myapp` to confirm current spend before raising.

Raising the cap does not restart services that auto-shutdown scaled to zero. They stay at zero until you run `convox budget reset myapp`, at any time after the raise.

## See Also

- [budget](/reference/cli/budget): full budget command group
- [budget reset](/reference/cli/budget-reset): acknowledge cap breach without raising
- [Cap raise](/management/budget-caps#raising-or-recovering-a-cap): operational guide
