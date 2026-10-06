---
title: "budget reset"
description: "The convox budget reset command acknowledges a cap breach and re-enables deploys, clearing the breaker and preserving the flap-prevention cooldown by default."
slug: budget-reset
url: /reference/cli/budget-reset
---
# budget reset

Acknowledge a cap breach and re-enable deploys. Clears the breaker and preserves the flap-prevention cooldown by default.

> **Plain reset requires the `rw` role.** A read-write user can run `convox budget reset` without admin role for the routine recovery flow.
>
> **`--force-clear-cooldown` requires admin role on the rack.** A non-admin caller (`rw` role) attempting `--force-clear-cooldown` receives `403 AppBudgetReset --force-clear-cooldown requires Admin role; current role is 'w'. Contact rack admin or use Admin token.` Basic-auth (rack-password) callers automatically pass the admin check.

### Usage
```bash
    convox budget reset <app> [--force] [--force-clear-cooldown]
```

### Examples

Reset the breaker on myapp; flap-suppress carry-over preserved:

```bash
    $ convox budget reset myapp
    This will acknowledge the current spend and re-enable deploys for myapp. Continue? [y/N]: y
    Resetting budget for myapp... OK
```

Reset and force-clear the flap-suppress cooldown so the next cap fire will NOT be suppressed (use only when you are sure the underlying cause is resolved):

```bash
    $ convox budget reset myapp --force-clear-cooldown
    This will acknowledge the current spend and re-enable deploys for myapp. Continue? [y/N]: y
    Resetting budget for myapp... OK
```

Reset without the confirmation prompt, for example in a script:

```bash
    $ convox budget reset myapp --force
    Resetting budget for myapp... OK
```

### Behavior

- Asks for confirmation unless `-f` or `--force` is passed. When stdin is not a terminal and `--force` is not passed, it exits with `refusing to prompt for confirmation on non-interactive stdin; pass --force to proceed`.
- Clears the breaker so deploys are re-enabled. Reset also clears the cap alert, so if spend is still at or above the cap, the next accumulator tick, within 10 minutes, fires the cap again and, under `block-new-deploys`, blocks deploys again.
- Restarts any services that were scaled down by an auto-shutdown. Raising the cap does not restart them; if you raise the cap, `convox budget reset` is still needed. Reset is the recovery path after an auto-shutdown fires.
- Preserves the flap-prevention cooldown unless `--force-clear-cooldown` is set. The flag is additive: it does not change the breaker-clear or service restart, it additionally clears the cooldown so the next cap fire is not suppressed.
- Does NOT reset the current month's spend; spend continues accumulating toward the cap. Reset clears the breaker, it does not zero out spend. The Console's Reset Period button also zeroes spend; the CLI has no equivalent.

For the full reset lifecycle see [Reset and force-clear cooldown](/management/budget-caps#reset-and-force-clear-cooldown).

## See Also

- [budget](/reference/cli/budget): full budget command group
- [budget cap raise](/reference/cli/budget-cap-raise): raise the cap instead of resetting
- [Reset and force-clear cooldown](/management/budget-caps#reset-and-force-clear-cooldown): operational guide
