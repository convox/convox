---
title: "budget cap raise"
description: "The convox budget cap raise command raises an app's monthly budget cap and clears the breaker in one operation when the new cap is above current spend."
slug: budget-cap-raise
url: /reference/cli/budget-cap-raise
---
# budget cap raise

Raise the monthly cap on an app's budget. Atomic with breaker-clear when the new cap is above both the previous cap and current spend. The command changes only the cap, like `convox budget set --monthly-cap N` with CLI `3.25.10` or later; earlier CLIs' `budget set --monthly-cap` also resets the alert threshold and at-cap action. Both `--monthly-cap` and `--monthly-cap-usd` are accepted; `--monthly-cap-usd` is the canonical name on this command.

> **Requires admin role on the rack.** A non-admin caller (`rw` role) receives `403 AppBudgetSet: admin role required to set budget cap`. Basic-auth (rack-password) callers automatically pass the admin check.

### Usage
```bash
    convox budget cap raise <app> --monthly-cap-usd N
```

### Examples

Raise from 250 USD to 500 USD on myapp; breaker clears:

```bash
    $ convox budget cap raise myapp --monthly-cap-usd 500
    Raising monthly cap for myapp... OK
```

A new cap below current spend is saved with a warning, and blocked deploys stay blocked because the cap trips again on the next accumulator tick:

```bash
    $ convox budget cap raise myapp --monthly-cap-usd 100
    WARNING: --monthly-cap-usd=$100.00 is below current month-to-date spend $134.65. Cap will trip immediately on next accumulator tick.
    Raising monthly cap for myapp... OK
```

When the new cap is above both the previous cap and current spend, raising it clears the breaker in the same operation, so there is no window where the cap is raised but deploys are still blocked. The same raise resets the threshold and cap alerts, so each can fire again this month against the new cap, and cancels an armed auto-shutdown countdown, which does not arm again unless spend crosses the new cap. Resetting the alerts when deploys are not blocked, and the countdown staying cancelled, need Rack version `3.25.10` or later.

Raising the cap does not restart services that auto-shutdown scaled to zero. They stay at zero until you run `convox budget reset myapp`, at any time after the raise.

For the full cap-raise lifecycle see [Cap raise](/management/budget-caps#raising-or-recovering-a-cap).

## See Also

- [budget cap](/reference/cli/budget-cap): parent command group
- [budget reset](/reference/cli/budget-reset): acknowledge cap breach without raising
- [Cap raise](/management/budget-caps#raising-or-recovering-a-cap): operational guide
