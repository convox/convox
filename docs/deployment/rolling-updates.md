---
title: "Rolling Updates"
description: "Rolling updates promote a Release with a make-one, break-one rollout that keeps services up and reverses automatically on health check failures."
slug: rolling-updates
url: /deployment/rolling-updates
---
# Rolling Updates

When a [Release](/reference/primitives/app/release) is promoted, new
[Processes](/reference/primitives/app/process) are gracefully rolled out
to avoid disruption to the [App](/reference/primitives/app).

## How it Works

The rolling update proceeds in a "make one, break one" process in order to maintain
[Service](/reference/primitives/app/service) uptime and capacity.

- Start 1 new [Process](/reference/primitives/app/process) on the new [Release](/reference/primitives/app/release)
- Verify that the new [Process](/reference/primitives/app/process) passes a [health check](/configuration/health-checks)
- Stop 1 old [Process](/reference/primitives/app/process) that is running the old [Release](/reference/primitives/app/release)
- Repeat until all [Processes](/reference/primitives/app/process) are running the new [Release](/reference/primitives/app/release)

## Minimum and Maximum Deployment Counts

Rolling updates will respect the [deployment configuration](/reference/primitives/app/service#deployment) to control the minimum number of healthy processes and maximum number of overall processes to have running at any one time during the update. This defaults to a minimum of 50% and a maximum of 200%.

```yaml
services:
  web:
    deployment:
      minimum: 50
      maximum: 200
```

These values are configured as percentages in the `deployment` section of your service definition in `convox.yml`. See the [Service](/reference/primitives/app/service) reference for all deployment options.

## Automatic Rollback

If any of the following conditions occur while the new [Release](/reference/primitives/app/release)
is being rolled out, the rollout will reverse and return all [Processes](/reference/primitives/app/process)
to the original [Release](/reference/primitives/app/release):

- A [Process](/reference/primitives/app/process) fails to start up and listen on the expected port
- A [Process](/reference/primitives/app/process) fails to pass a [health check](/configuration/health-checks)

Any of these issues will appear in the logs that display during the promotion to help you determine
what is going wrong.

See [Troubleshooting](/help/troubleshooting) for some tips on diagnosing common failure cases, or run [deploy-debug](/reference/cli/deploy-debug) to inspect the failing pods directly.

## Failure Detection

By default a broken rollout is failed by the App's own rollout timeout, which takes 50 minutes. Two per-Service settings shorten that.

```yaml
services:
  web:
    deployment:
      progressDeadline: 600
      crashRestartLimit: 5
```

`progressDeadline` is the number of seconds the rollout may go without progress. A rollout makes progress whenever a new [Process](/reference/primitives/app/process) becomes ready, and the clock resets on every such event, so a large rollout that is advancing one Process at a time is never cut short. Once the deadline passes with no progress, the rollout is failed and the previous [Release](/reference/primitives/app/release) is restored.

`crashRestartLimit` is the number of container restarts the rollout may accumulate. A Process that becomes ready and only then starts crashing keeps making progress, so `progressDeadline` never trips on it. Set `crashRestartLimit` to catch a crash loop.

Both settings are off by default.

### Choosing Values

`progressDeadline` has to exceed the slowest healthy start-up time of the Service, or a healthy deploy will be rolled back. GPU Services are configured for up to 35 minutes of cold start by default. A value of 3000 or higher arms no detection; it only raises the App's rollout timeout.

`crashRestartLimit` does not map evenly onto wall clock, because Kubernetes waits ten seconds before the first restart and doubles that wait up to a five-minute cap. Restart 3 lands at roughly a minute, restart 5 at roughly five minutes, and restart 10 at roughly thirty minutes. A limit of N aborts on restart N+1, so a limit of 10 lands around the `convox deploy` command's own 35-minute ceiling. Three to five is a reasonable range.

### What Each Setting Covers

`ImagePullBackOff`, `ErrImagePull` and `CreateContainerConfigError` never increment a restart count, so they are covered by `progressDeadline` only.

Agent Services run as DaemonSets and stateful Services run as StatefulSets. Neither kind carries a rollout progress deadline, so `progressDeadline` renders nothing on those workloads and `crashRestartLimit` is the only fast-failure mechanism available for them. A stateful Service's `progressDeadline` still counts toward the App's rollout timeout, so setting one above 3000 extends how long the whole App may take to converge.

A Process that never gets scheduled at all, for example because a volume cannot be bound, is covered by neither setting.

### Rack-Wide Defaults

An operator can turn either check on for every App on a Rack with the [deploy_progress_deadline](/configuration/rack-parameters/aws/deploy_progress_deadline) and [deploy_crash_restart_limit](/configuration/rack-parameters/aws/deploy_crash_restart_limit) Rack parameters. A Service's own `convox.yml` wins over the Rack parameter, which wins over the shipped default.

| To turn off | Supported way |
|-------------|---------------|
| Rack-wide | Set `deploy_progress_deadline=0` and `deploy_crash_restart_limit=0`, the shipped default |
| One Service, against a Rack default | `crashRestartLimit: -1` in `convox.yml` |
| One Service's own setting | Remove the `deployment.progressDeadline` or `deployment.crashRestartLimit` field |

There is no per-Service opt-out from a Rack-wide `deploy_progress_deadline`. A Service can raise or lower its own deadline, but not remove it.

## See Also

- [Health Checks](/configuration/health-checks) for configuring how Convox verifies new processes
- [Rollbacks](/deployment/rollbacks) for reverting a failed deployment
- [deploy-debug](/reference/cli/deploy-debug) for diagnosing why a rollout was reversed
- [deploy_progress_deadline](/configuration/rack-parameters/aws/deploy_progress_deadline) and [deploy_crash_restart_limit](/configuration/rack-parameters/aws/deploy_crash_restart_limit) for setting failure detection Rack-wide
