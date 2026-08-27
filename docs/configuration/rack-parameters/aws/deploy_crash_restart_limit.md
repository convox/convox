---
title: "deploy_crash_restart_limit"
description: "The deploy_crash_restart_limit AWS rack parameter aborts a rollout once a container has restarted more than the given number of times, disabled by default."
slug: deploy_crash_restart_limit
url: /configuration/rack-parameters/aws/deploy_crash_restart_limit
---

# deploy_crash_restart_limit

## Description
The `deploy_crash_restart_limit` parameter sets the Rack-wide default number of container restarts a Service may accumulate during a rollout before Convox aborts the deploy and restores the previous Release.

It covers the failures [deploy_progress_deadline](/configuration/rack-parameters/aws/deploy_progress_deadline) cannot. The deadline catches a Service that never becomes ready. The restart limit catches one that becomes ready, crashes, and keeps crashing.

Any Service can override this in its own `convox.yml`:

```yaml
services:
  web:
    deployment:
      crashRestartLimit: 5
```

The Service value wins, then this parameter, then off. A Service value of `0` means "use the Rack value", a positive number sets that Service's own limit, and `-1` opts the Service out of a Rack-wide limit entirely.

## Default Value
The default value for `deploy_crash_restart_limit` is `0`, which disables the check.

## Use Cases
- **Crash loop detection**: Abort a deploy whose new container fails a start-up migration instead of running out the App's 50-minute rollout timeout.
- **Rack-wide rollout**: Turn the check on for every App without editing any `convox.yml`.

## Setting Parameters
```bash
$ convox rack params set deploy_crash_restart_limit=3 -r rackName
Updating parameters... OK
```

Accepted values are `0` and any positive number of restarts. There is no upper bound, though values past `10` fire after the `convox deploy` command has already stopped waiting.

## Choosing a Value
Restart number does not map evenly onto wall clock. Kubernetes waits ten seconds before the first restart and doubles that wait up to a five-minute cap, so restart 3 lands at roughly a minute, restart 5 at roughly five minutes, and restart 10 at roughly thirty minutes. A limit of N aborts on restart N+1, so a limit of 10 fires around the 35-minute mark.

| Value | Effect |
|-------|--------|
| `0` | Disabled, the shipped default. Container restarts never abort a rollout. |
| `3` to `5` | Aborts a crash-looping deploy within a few minutes. A good starting point. |
| `10` or more | Fires at or beyond the `convox deploy` command's own 35-minute ceiling, so it offers little over the default. |

## Turning Detection Off

| To turn off | Supported way |
|-------------|---------------|
| Rack-wide | Set `deploy_crash_restart_limit=0` and `deploy_progress_deadline=0`, the shipped default |
| One Service, against a Rack default | `crashRestartLimit: -1` in `convox.yml` |
| One Service's own setting | Remove the `deployment.crashRestartLimit` or `deployment.progressDeadline` field |

The `-1` opt-out applies to this parameter only. There is no per-Service opt-out from a Rack-wide `deploy_progress_deadline`.

## Additional Information
- Restarts are counted per container and the highest count in the Pod is the one compared, so a Service with several sidecars does not trip the limit faster than a single-container one.
- Init container restarts count toward the limit, which covers a failing boot-time migration.
- Only Pods started by the current rollout are counted. Pods left over from the previous Release are ignored, so `convox apps params set` or `convox apps lock` on a long-running App with a flaky sidecar does not roll the App back.
- This check covers agent and stateful Services as well as ordinary ones. A Rack-wide limit therefore applies to log shippers too. Set `crashRestartLimit: -1` on any Service that should be exempt.
- `ImagePullBackOff`, `ErrImagePull` and `CreateContainerConfigError` never increment the restart count. [deploy_progress_deadline](/configuration/rack-parameters/aws/deploy_progress_deadline) covers those.

## See Also
- [deploy_progress_deadline](/configuration/rack-parameters/aws/deploy_progress_deadline) for failing a rollout that stops making progress
- [Rolling Updates: Failure Detection](/deployment/rolling-updates#failure-detection) for how the two checks divide the failure cases
- [Service: deployment](/reference/primitives/app/service#deployment) for the per-Service `convox.yml` fields

## Version Requirements
This parameter requires at least Convox Rack version `3.25.5`.
