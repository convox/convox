---
title: "release_watcher_gc_interval"
slug: release_watcher_gc_interval
url: /configuration/rack-parameters/aws/release_watcher_gc_interval
---

# release_watcher_gc_interval

## Description
The `release_watcher_gc_interval` parameter controls how frequently the rack's release-watcher subsystem runs garbage collection on orphaned watcher slots. The watcher tracks in-flight `convox releases promote` operations; under heavy promote churn (many promotes within a short window), orphaned watcher slots can accumulate if not periodically swept.

The value is plumbed through Terraform (`system → rack → api`) and surfaces as the `RELEASE_WATCHER_GC_INTERVAL` environment variable on the api Deployment. The provider package reads the env var once at Initialize and writes it to the package-level GC tick interval.

Accepts a Go-style duration string — for example `5m`, `30m`, or `1h`.

## Default Value
The default value is `5m`.

## Allowed Range
`60s` to `1h`. Values below `60s` produce excessive Kubernetes API churn (200 apps × 60s sweep = 12000 ops/min for GC alone); values above `1h` allow orphaned slots to accumulate beyond reasonable bounds. The validator at `pkg/cli/rack.go` rejects out-of-range values.

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

To clear the override (falls back to the package default `5m`):
```bash
$ convox rack params set release_watcher_gc_interval= -r rackName
Setting parameters... OK
```

## Operational Notes
- The provider reads `RELEASE_WATCHER_GC_INTERVAL` once at Initialize. Changing the value triggers an api Deployment rolling restart via the `convox.com/secret-checksum-*` annotation hash mechanism on the api pod template.
- Invalid values (unparseable durations, out-of-range) fall back to the default `5m` and emit a warning log; a parsing failure does not crash the api Deployment.
- The GC sweep itself is bounded — even at the maximum `1h` interval, accumulation is capped because individual watcher slots have their own per-slot expiry.

## Related Parameters
- [releases_to_retain_after_active](/configuration/rack-parameters/aws/releases_to_retain_after_active): Controls how many historical releases the rack retains in storage. Independent of the GC sweep frequency for in-flight watcher slots, but conceptually adjacent.

## Version Requirements
This parameter requires at least Convox rack version `3.24.6`.
