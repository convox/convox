---
title: "deploy_progress_deadline"
description: "The deploy_progress_deadline AWS rack parameter sets how long a service rollout may go without progress before Kubernetes marks it failed, unset by default."
slug: deploy_progress_deadline
url: /configuration/rack-parameters/aws/deploy_progress_deadline
---

# deploy_progress_deadline

## Description
The `deploy_progress_deadline` parameter sets the rack-wide default for how many seconds a service rollout may go without making progress before Kubernetes declares it failed. When that happens Convox aborts the rollout, restores the previous release, and sends the usual failure notification, in minutes rather than after the full 50-minute default.

A rollout makes progress whenever a new copy of the service becomes ready. The clock resets on every such event, so a large rollout that is genuinely advancing one copy at a time is never cut short.

Any service can override this in its own `convox.yml`:

```yaml
services:
  web:
    deployment:
      progressDeadline: 600
```

The service value wins, then this parameter, then unset.

## Default Value
The default value for `deploy_progress_deadline` is `0`, which leaves the deadline unset. Leaving it alone changes nothing: Kubernetes applies its own default and the Rack's 3000-second rollout timeout remains the only failure path, exactly as before.

## Use Cases
- **Fast failure feedback**: Lower the value so a broken deploy fails while the developer who ran it is still watching.
- **Slow-starting services**: Raise the value above 3000 for a fleet that legitimately needs longer, so a healthy rollout is not cut off.
- **Rack-wide rollout**: Turn on fast failure detection for every app on the Rack without editing any `convox.yml`.

## Setting Parameters
```bash
$ convox rack params set deploy_progress_deadline=600 -r rackName
Updating parameters... OK
```

Accepted values are `0`, or `30` through `21600`. `0` leaves the deadline unset, which is the default.

## Choosing a Value
The value has to exceed the slowest healthy start-up time of any service on the Rack, or healthy deploys will be rolled back.

| Value | Effect |
|-------|--------|
| 0 | Unset, the shipped default. A broken deploy fails on the Rack's own 3000-second timeout. |
| 300 to 600 | Fails a broken deploy in five to ten minutes. Suitable when every service starts quickly. |
| Above 3000 | Gives slow-starting services more room. Also raises how long a rollback may take to converge. |

## Additional Information
- GPU services get a start-up probe that allows up to 30 minutes of cold start by default, so a Rack running them should not be set below that.
- Agent services are DaemonSets and stateful services are StatefulSets. Neither kind has a progress deadline, so this parameter does not apply to them. Use `deployment.crashRestartLimit` for those.
- A value above 2100 means the `convox deploy` command gives up before the Rack reaches a verdict. The deploy still finishes, or still rolls back, on the Rack.
- This parameter measures time without progress. A service that becomes ready and then crashes never trips it; `deploy_crash_restart_limit` covers that case.
- To turn detection off for the whole Rack without changing any app, add `deploy-fast-fail-disable=true` to `api_feature_gates`. Read the current value first and set the full comma-separated list, because that parameter is replaced rather than appended to.
