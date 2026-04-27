---
title: "budget cap raise"
slug: budget-cap-raise
url: /reference/cli/budget-cap-raise
---
# budget cap raise

Raise the monthly cap on an app's budget. Atomic with breaker-clear when the
new cap is above current spend. Alias for `convox budget set --monthly-cap N`.

### Usage
```bash
    convox budget cap raise [-a app] --monthly-cap N
```

### Examples

Raise from 250 USD to 500 USD on myapp; breaker clears:

```bash
    $ convox budget cap raise --app myapp --monthly-cap 500
    Raising monthly cap to 500.00 USD... OK
    Breaker cleared.
```

Raise rejected when the new cap is below current spend:

```bash
    $ convox budget cap raise --app myapp --monthly-cap 100
    error: new cap 100.00 USD is below current spend 134.65 USD
```

The cap-raise + breaker-clear pair is atomic. The rack acquires the per-app
lock, persists the new cap, and clears the `CircuitBreakerTripped` flag plus
the alert-fired timestamps in the same critical section. There is no observable
window where the new cap is set but the old breaker is still tripped.

After `:fired` (post-shutdown), cap-raise clears the breaker but does NOT
restart already-shutdown services. Run `convox budget reset myapp` to
restore replicas from the persisted shutdown-state annotation.

A cap-raise that clears the breaker emits the `app:budget:breaker-cleared`
audit event in 3.24.6+ (top-level event, not a sub-type of
`auto-shutdown`). Receivers parsing webhook events should fail-open on
unknown event types or be updated to handle the new type. See
[Webhook Signing](/console/webhook-signing#receiver-migration).

## See Also

- [budget cap](/reference/cli/budget-cap) — parent command group
- [budget reset](/reference/cli/budget-reset) — acknowledge cap breach without raising
- [Cap raise](/management/budget-caps#cap-raise) — operational guide
- [Webhook Signing](/console/webhook-signing) — `:breaker-cleared` event semantics
