---
title: "budget dismiss-recovery"
description: "The convox budget dismiss-recovery command dismisses the sticky recovery banner shown after an auto-shutdown recovers, like the Console Dismiss button."
slug: budget-dismiss-recovery
url: /reference/cli/budget-dismiss-recovery
---
# budget dismiss-recovery

Dismiss the sticky recovery banner that displays after an auto-shutdown
recovers. Equivalent to clicking "Dismiss" in the Console banner.

### Usage
```bash
    convox budget dismiss-recovery <app>
```

### Examples

Dismiss the active recovery banner on myapp:

```bash
    $ convox budget dismiss-recovery myapp
    Banner dismissed for myapp.
```

Idempotent. A second call is a no-op:

```bash
    $ convox budget dismiss-recovery myapp
    Banner already dismissed for myapp.
```

No banner present:

```bash
    $ convox budget dismiss-recovery myapp
    No recovery banner active for myapp.
```

### Behavior

- The dismiss is **per-app**, not per-user. Once dismissed by any user the
  banner hides for everyone viewing that app.
- The dismiss applies only to the current recovery cycle; a later
  auto-shutdown and recovery shows a fresh banner. If a banner is stuck from
  an earlier cycle, see the
  [troubleshooting recipe](/management/budget-caps#troubleshooting).
- The dismiss is recorded in the app's audit trail with the user who
  dismissed it.

## See Also

- [budget](/reference/cli/budget): full budget command group
- [Recovery banner persistence](/management/budget-caps#troubleshooting):
  recovery for racks already in stuck state
