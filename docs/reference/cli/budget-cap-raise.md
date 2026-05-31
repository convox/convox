---
title: "budget cap raise"
slug: budget-cap-raise
url: /reference/cli/budget-cap-raise
---
# budget cap raise

Raise the monthly cap on an app's budget. Atomic with breaker-clear when the
new cap is above current spend. Alias for `convox budget set --monthly-cap N`
(both `--monthly-cap` and `--monthly-cap-usd` are accepted forms; `--monthly-cap-usd`
is the canonical name on this command).

> **Requires admin role on the rack.** A non-admin caller (`rw` role)
> receives `403 AppBudgetSet: admin role required to set budget cap`.
> Basic-auth (rack-password) callers automatically pass the admin check.

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

Raise rejected when the new cap is below current spend:

```bash
    $ convox budget cap raise myapp --monthly-cap-usd 100
    error: new cap 100.00 USD is below current spend 134.65 USD
```

When the new cap is above current spend, raising it clears the breaker in the
same operation, so there is no window where the cap is raised but deploys are
still blocked.

After an auto-shutdown, cap-raise clears the breaker but does NOT restart
already-shutdown services. Run `convox budget reset myapp` to restart them.

For the full cap-raise lifecycle see [Cap raise](/management/budget-caps#cap-raise).

## See Also

- [budget cap](/reference/cli/budget-cap): parent command group
- [budget reset](/reference/cli/budget-reset): acknowledge cap breach without raising
- [Cap raise](/management/budget-caps#cap-raise): operational guide
