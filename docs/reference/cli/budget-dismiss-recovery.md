---
title: "budget dismiss-recovery"
slug: budget-dismiss-recovery
url: /reference/cli/budget-dismiss-recovery
---
# budget dismiss-recovery

Dismiss the sticky recovery banner that displays after an auto-shutdown
recovers. Equivalent to clicking "Dismiss" in the Console banner.

### Usage
```bash
    convox budget dismiss-recovery [-a app]
```

### Examples

Dismiss the active recovery banner on myapp:

```bash
    $ convox budget dismiss-recovery --app myapp
    Dismissing recovery banner for myapp... OK
```

Idempotent — second call is a no-op:

```bash
    $ convox budget dismiss-recovery --app myapp
    Recovery banner already dismissed for myapp
```

No banner present:

```bash
    $ convox budget dismiss-recovery --app myapp
    No recovery banner present for myapp
```

### Behavior

- The dismiss is **per-app**, not per-user. Once dismissed by any user the
  banner hides for everyone viewing that app.
- The dismiss timestamp is stored as a namespace annotation. The
  stale-annotation GC clears it one tick after the underlying shutdown-state
  annotation passes terminal-state, so cycle-N's dismiss never leaks into
  cycle-N+1's RECOVERED banner. (Pre-3.24.6 racks had a leak that has been
  fixed; for stuck pre-fix racks see the [troubleshooting
  recipe](/management/budget-caps#troubleshooting).)
- Emits the `app:budget:auto-shutdown:dismissed` audit event with the
  authenticated `actor`. Receivers ingesting auto-shutdown events should
  treat this as audit-only (not part of the 9 lifecycle events). See
  [Webhook Signing](/console/webhook-signing#receiver-migration).
- Concurrent dismiss clicks (e.g. two operators clicking simultaneously) are
  serialized via the per-app lock; only one fresh `:dismissed` event fires,
  the second observes `Status="already-dismissed"`.

## See Also

- [budget](/reference/cli/budget) — full budget command group
- [Recovery banner persistence](/management/budget-caps#troubleshooting) —
  recovery for racks already in stuck state
- [ack_by Derivation](/migration/ack-by-derivation) — actor field semantics
