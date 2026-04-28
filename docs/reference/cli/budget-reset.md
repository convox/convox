---
title: "budget reset"
slug: budget-reset
url: /reference/cli/budget-reset
---
# budget reset

Acknowledge a cap breach and re-enable deploys. Clears the breaker. Preserves
flap-suppress carry-over by default.

### Usage
```bash
    convox budget reset [-a app] [--force-clear-cooldown]
```

### Examples

Reset the breaker on myapp; flap-suppress carry-over preserved:

```bash
    $ convox budget reset --app myapp
    Resetting budget for myapp... OK
    Breaker cleared.
```

Reset and force-clear the flap-suppress cooldown so the next cap fire will
NOT be suppressed (use only when you are sure the underlying cause is
resolved):

```bash
    $ convox budget reset --app myapp --force-clear-cooldown
    Resetting budget for myapp (force-clearing flap-suppress cooldown)... OK
```

### Behavior

- Clears `CircuitBreakerTripped` and the alert-fired timestamps.
- Restarts services that were already shutdown by `:fired` (calls
  `restoreFromAnnotation` to reapply the persisted replica counts).
  Both the plain reset and `--force-clear-cooldown` invoke the same
  replica-restore path; cap-raise alone clears the breaker but does NOT
  restart services, so `convox budget reset` is the canonical post-`:fired`
  recovery path.
- Preserves the flap-suppress carry-over (`FlapSuppressedUntil` and
  `FlapSuppressFiredAt` annotations) unless `--force-clear-cooldown` is set.
  The flag is additive — it does not change the breaker-clear or replica
  restore behavior, but it additionally clears the flap-suppress cooldown
  so the next cap fire will not be suppressed.
- Emits `app:budget:auto-shutdown:cancelled` reason=`reset-during-armed` if
  the reset interrupts an in-flight armed countdown.
- Does NOT reset the current month's spend; spend continues accumulating
  toward the cap. Reset is the breaker-clear, not a spend-zero.

The reset is serialized against the accumulator's tick path via the per-app
lock — concurrent reset and accumulator decisions cannot produce duplicate
`:cancelled` events.

## See Also

- [budget](/reference/cli/budget) — full budget command group
- [budget cap raise](/reference/cli/budget-cap-raise) — raise the cap instead
  of resetting
- [Reset and force-clear cooldown](/management/budget-caps#force-clear-cooldown) — operational guide
