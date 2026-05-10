---
title: "budget cap"
slug: budget-cap
url: /reference/cli/budget-cap
---
# budget cap

The `budget cap` command group operates on the `monthlyCapUsd` field of an
app's budget config. Currently exposes a single subcommand: `raise`. Lowering
or removing the cap is done through `convox budget set` or `convox budget clear`.

## budget cap raise

Raise the monthly cap. Atomic with breaker-clear when the new cap is above
current spend.

> **Requires admin role on the rack RBAC.** Mutating the monthly cap is
> admin-gated end-to-end (`AppBudgetSet` cap-mutation path on the rack).
> A non-admin caller (`rw` role) receives `403 AppBudgetSet: admin role
> required to set budget cap`. Basic-auth (rack-password) callers
> automatically pass the admin check.

### Usage
```bash
    convox budget cap raise [-a app] --monthly-cap-usd N
```

### Examples
```bash
    $ convox budget cap raise --app myapp --monthly-cap-usd 500
    Raising monthly cap to 500.00 USD... OK
    Breaker cleared.
```

If the new cap is below current spend, the request is rejected:

```bash
    $ convox budget cap raise --app myapp --monthly-cap-usd 100
    error: new cap 100.00 USD is below current spend 134.65 USD
```

Use `convox cost --app myapp` to confirm current spend before raising.

After `:fired` (post-shutdown), cap-raise clears the breaker but does NOT
restart already-shutdown services. Run `convox budget reset myapp` to
restore replicas from the persisted shutdown-state annotation.

## See Also

- [budget](/reference/cli/budget) — full budget command group
- [budget reset](/reference/cli/budget-reset) — acknowledge cap breach without raising
- [Cap raise](/management/budget-caps#cap-raise) — operational guide
