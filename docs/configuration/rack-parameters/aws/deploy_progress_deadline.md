---
title: "deploy_progress_deadline"
description: "The deploy_progress_deadline AWS rack parameter sets how long a Service rollout may go without progress before Kubernetes marks it failed, unset by default."
slug: deploy_progress_deadline
url: /configuration/rack-parameters/aws/deploy_progress_deadline
---

# deploy_progress_deadline

## Description
The `deploy_progress_deadline` parameter sets the Rack-wide default for how many seconds a Service rollout may go without making progress before Kubernetes declares it failed. Convox then aborts the rollout, restores the previous Release, and sends the usual failure notification. The deploy fails in minutes instead of running out the 50-minute default.

A rollout makes progress whenever a new Process becomes ready. The clock resets on every such event, so a large rollout that is advancing one Process at a time is never cut short.

Any Service can override this in its own `convox.yml`:

```yaml
services:
  web:
    deployment:
      progressDeadline: 600
```

The Service value wins, then this parameter, then unset. A Service can raise or lower its own deadline but cannot opt out of a Rack-wide one: `progressDeadline: 0` means "use the Rack value", and the `-1` opt-out belongs to [deploy_crash_restart_limit](/configuration/rack-parameters/aws/deploy_crash_restart_limit) alone.

## Default Value
The default value for `deploy_progress_deadline` is `0`, which leaves the deadline unset. Kubernetes applies its own default and the App's 50-minute rollout timeout stays the only failure path.

## Use Cases
- **Fast failure feedback**: Lower the value so a broken deploy fails while the developer who ran it is still watching.
- **Rack-wide rollout**: Turn on failure detection for every App on the Rack without editing any `convox.yml`.
- **Longer rollout timeout**: Raise the value above 3000 to give the whole App more time to converge. Failure detection is not armed at those values.

## Setting Parameters
```bash
$ convox rack params set deploy_progress_deadline=600 -r rackName
Updating parameters... OK
```

Accepted values are `0`, or `30` through `21600`. `0` leaves the deadline unset, which is the default.

## Choosing a Value
The value has to exceed the slowest healthy start-up time of any Service on the Rack, or healthy deploys will be rolled back.

Failure detection is armed only below 3000 seconds. At 3000 and above, no progress deadline is rendered on the workload and nothing fails the rollout early.

| Value | Effect |
|-------|--------|
| `0` | Unset, the shipped default. A broken deploy fails on the App's 50-minute rollout timeout. |
| `30` to `2999` | Detection armed. A broken deploy fails this many seconds after the rollout last made progress. |
| `3000` to `21600` | No detection. Above 3000, raises the App's rollout timeout to this many seconds. |

A value of 300 to 600 fails a broken deploy in five to ten minutes and suits a Rack where every Service starts quickly.

## App Rollout Timeout
The App's rollout timeout is 3000 seconds by default. The largest effective `progressDeadline` across the Release's non-agent Services replaces it when that value is larger. One Service set to `progressDeadline: 7200` therefore extends the rollout timeout for the entire App, including Services that start in seconds.

Stateful Services count toward this maximum even though they carry no progress deadline of their own.

## Turning Detection Off

| To turn off | Supported way |
|-------------|---------------|
| Rack-wide | Set `deploy_progress_deadline=0` and `deploy_crash_restart_limit=0`, the shipped default |
| One Service, against a Rack default | `crashRestartLimit: -1` in `convox.yml` |
| One Service's own setting | Remove the `deployment.progressDeadline` or `deployment.crashRestartLimit` field |

There is no per-Service opt-out from a Rack-wide `deploy_progress_deadline`.

## Additional Information
- GPU Services get a start-up probe that allows up to 35 minutes of cold start by default, so a Rack running them should not set this below `2100`.
- Agent Services are DaemonSets and stateful Services are StatefulSets. Neither kind carries a progress deadline, so this parameter renders nothing on those workloads. Use `deployment.crashRestartLimit` to fail them fast.
- Above `2100`, the `convox deploy` command reaches its own 35-minute ceiling before the deadline can fail the rollout. The Release still finishes, or still rolls back, on the Rack.
- This parameter measures time without progress. A Service that becomes ready and then crashes keeps making progress, so it never trips the deadline. [deploy_crash_restart_limit](/configuration/rack-parameters/aws/deploy_crash_restart_limit) covers that case.

## See Also
- [deploy_crash_restart_limit](/configuration/rack-parameters/aws/deploy_crash_restart_limit) for aborting a rollout on repeated container restarts
- [Rolling Updates: Failure Detection](/deployment/rolling-updates#failure-detection) for how the two checks divide the failure cases
- [Service: deployment](/reference/primitives/app/service#deployment) for the per-Service `convox.yml` fields

## Version Requirements
This parameter requires at least Convox Rack version `3.25.5`.
