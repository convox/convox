---
title: "release_watcher_gc_interval"
slug: release_watcher_gc_interval
url: /configuration/rack-parameters/aws/release_watcher_gc_interval
---

# release_watcher_gc_interval

## Description
The `release_watcher_gc_interval` parameter controls how frequently the rack cleans up orphaned tracking slots for in-flight promote operations. The rack tracks each in-flight `convox releases promote`; under heavy promote churn (many promotes within a short window), orphaned slots can accumulate if not periodically swept.

Accepts a duration string, for example `5m`, `30m`, or `1h`.

## Default Value
The default value is `5m`.

## Allowed Range
`60s` to `1h`. Values below `60s` produce excessive Kubernetes API churn on racks with many apps; values above `1h` allow orphaned slots to accumulate beyond reasonable bounds. Values outside the `60s` to `1h` range, or values that are not valid durations, are rejected.

## Use Cases
- **High-churn deployment pipelines**: Bump to `2m` for racks running CI/CD with frequent rolling promotes; tighter sweep prevents orphan accumulation.
- **Quiet steady-state racks**: Bump to `30m` or `1h` to reduce baseline Kubernetes API load on racks with infrequent promote activity.

## Setting Parameters
To tighten the GC sweep to 2 minutes:
```bash
$ convox rack params set release_watcher_gc_interval=2m -r rackName
Setting parameters... OK
```

To revert to the default:
```bash
$ convox rack params set release_watcher_gc_interval=5m -r rackName
Setting parameters... OK
```

To clear the override (falls back to the default `5m`):
```bash
$ convox rack params set release_watcher_gc_interval= -r rackName
Setting parameters... OK
```

## Operational Notes
- Changing this value triggers a rolling restart of the rack's API component so the new interval takes effect.
- The cleanup sweep is bounded. Even at the maximum `1h` interval, accumulation is capped because individual tracking slots have their own expiry.

## Related Parameters
- [releases_to_retain_after_active](/configuration/rack-parameters/aws/releases_to_retain_after_active): Controls how many historical releases the rack retains in storage. Independent of the GC sweep frequency for in-flight watcher slots, but conceptually adjacent.

## Version Requirements
This parameter requires at least Convox rack version `3.24.6`.
