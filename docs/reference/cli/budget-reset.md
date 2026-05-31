---
title: "budget reset"
description: "The convox budget reset command acknowledges a cap breach and re-enables deploys, clearing the breaker and preserving the flap-prevention cooldown by default."
slug: budget-reset
url: /reference/cli/budget-reset
---
# budget reset

Acknowledge a cap breach and re-enable deploys. Clears the breaker and
preserves the flap-prevention cooldown by default.

> **Plain reset requires the `rw` role.** A read-write user can run
> `convox budget reset` without admin role for the routine recovery flow.
>
> **`--force-clear-cooldown` requires admin role on the rack.** A non-admin
> caller (`rw` role) attempting `--force-clear-cooldown` receives
> `403 AppBudgetReset --force-clear-cooldown requires Admin role; current
> role is 'w'. Contact rack admin or use Admin token.` Basic-auth
> (rack-password) callers automatically pass the admin check.

### Usage
```bash
    convox budget reset <app> [--force-clear-cooldown]
```

### Examples

Reset the breaker on myapp; flap-suppress carry-over preserved:

```bash
    $ convox budget reset myapp
    Resetting budget for myapp... OK
```

Reset and force-clear the flap-suppress cooldown so the next cap fire will
NOT be suppressed (use only when you are sure the underlying cause is
resolved):

```bash
    $ convox budget reset myapp --force-clear-cooldown
    Resetting budget for myapp (force-clearing flap-suppress cooldown)... OK
```

### Behavior

- Clears the breaker so deploys are re-enabled.
- Restarts any services that were scaled down by an auto-shutdown. Cap-raise
  alone clears the breaker but does NOT restart services, so `convox budget
  reset` is the recovery path after an auto-shutdown fires.
- Preserves the flap-prevention cooldown unless `--force-clear-cooldown` is
  set. The flag is additive: it does not change the breaker-clear or service
  restart, it additionally clears the cooldown so the next cap fire is not
  suppressed.
- Does NOT reset the current month's spend; spend continues accumulating
  toward the cap. Reset clears the breaker, it does not zero out spend.

For the full reset lifecycle see
[Reset and force-clear cooldown](/management/budget-caps#force-clear-cooldown).

## See Also

- [budget](/reference/cli/budget): full budget command group
- [budget cap raise](/reference/cli/budget-cap-raise): raise the cap instead
  of resetting
- [Reset and force-clear cooldown](/management/budget-caps#force-clear-cooldown): operational guide
