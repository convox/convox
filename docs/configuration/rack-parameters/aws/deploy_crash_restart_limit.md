---
title: "deploy_crash_restart_limit"
description: "The deploy_crash_restart_limit AWS rack parameter aborts a rollout once a container has restarted more than the given number of times, disabled by default."
slug: deploy_crash_restart_limit
url: /configuration/rack-parameters/aws/deploy_crash_restart_limit
---

# deploy_crash_restart_limit

## Description
The `deploy_crash_restart_limit` parameter sets the rack-wide default number of container restarts a service may accumulate during a rollout before Convox aborts the deploy and restores the previous release.

It complements `deploy_progress_deadline`. The deadline catches a service that never becomes ready; the restart limit catches one that starts, crashes, and keeps crashing.

Any service can override this in its own `convox.yml`:

```yaml
services:
  web:
    deployment:
      crashRestartLimit: 5
```

The service value wins, then this parameter, then off. A service can opt out of a rack-wide limit entirely with `crashRestartLimit: -1`.

## Default Value
The default value for `deploy_crash_restart_limit` is `0`, which disables the check.

## Use Cases
- **Crash loop detection**: Abort a deploy whose new container fails a start-up migration instead of waiting out the Rack's timeout.
- **Rack-wide rollout**: Turn the check on for every app without editing any `convox.yml`.

## Setting Parameters
```bash
$ convox rack params set deploy_crash_restart_limit=3 -r rackName
Updating parameters... OK
```

Accepted values are `0` and above. `0` disables the check.

## Choosing a Value
Restart number does not map evenly onto wall clock. Kubernetes waits ten seconds before the first restart and doubles that wait up to a five-minute cap, so restart 3 lands at roughly a minute, restart 5 at roughly five minutes, and restart 10 at roughly thirty minutes. A limit of N aborts on restart N+1, so a limit of 10 fires around the 35-minute mark.

| Value | Effect |
|-------|--------|
| 3 to 5 | Aborts a crash-looping deploy within a few minutes. A good starting point. |
| 10 or more | Fires at or beyond the `convox deploy` command's own 35-minute ceiling, so it offers little over the default. |

## Additional Information
- Restarts are counted per container and the highest count in the pod is the one compared, so a service with several sidecars does not trip the limit faster than a single-container one.
- Init container restarts count, which is what catches a failing boot-time migration.
- Pods left over from the previous release are ignored, so `convox apps params set` and `convox apps lock` on a long-running app with a flaky sidecar do not roll it back.
- This check covers agent and stateful services as well as ordinary ones. A rack-wide limit therefore applies to log shippers too; use `crashRestartLimit: -1` on any service that should be exempt.
- `ImagePullBackOff`, `ErrImagePull` and `CreateContainerConfigError` never increment the restart count. Those are covered by `deploy_progress_deadline`.
- To turn detection off for the whole Rack without changing any app, add `deploy-fast-fail-disable=true` to `api_feature_gates`. Read the current value first and set the full comma-separated list, because that parameter is replaced rather than appended to.
