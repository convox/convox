---
title: "Rolling Updates"
draft: false
slug: Rolling Updates
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

## Minimum / Maximum configuration

Rolling updates will respect the [deployment configuration](/reference/primitives/app/service#deployment) to control the minimum number of healthy processes and maximum number of overall processes to have running at any one time during the update.  This defaults to a minimum of 50% and a maximum of 200%.

## Automatic Rollback

If any of the following conditions occur while the new [Release](/reference/primitives/app/release)
is being rolled out, the rollout will reverse and return all [Processes](/reference/primitives/app/process)
to the original [Release](/reference/primitives/app/release):

- A [Process](/reference/primitives/app/process) fails to start up and listen on the expected port
- A [Process](/reference/primitives/app/process) fails to pass a [health check](/configuration/health-checks)

Any of these issues will appear in the logs that display during the promotion to help you determine
what is going wrong.

See [Troubleshooting](/help/troubleshooting) for some tips on diagnosing common failure cases.