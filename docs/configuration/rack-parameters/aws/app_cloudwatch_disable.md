---
title: "app_cloudwatch_disable"
description: "The app_cloudwatch_disable AWS rack parameter stops both CloudWatch writers for the per-App log groups while leaving the Rack system group live, defaulting to false."
slug: app_cloudwatch_disable
url: /configuration/rack-parameters/aws/app_cloudwatch_disable
---

# app_cloudwatch_disable

## Description

The `app_cloudwatch_disable` parameter stops both CloudWatch writers for the per-App log groups `/convox/<rack>/<app>`. The Rack controller stops writing Kubernetes events and deploy state transitions for Apps, and Fluentd stops shipping App container output. The Rack system group `/convox/<rack>/system` is exempt from both stops, so `convox rack logs` keeps working and the Nginx access log stream keeps arriving.

The name looks like a narrower `cloudwatch_disable`, and the two parameters do not stand in that relation. `app_cloudwatch_disable` covers fewer groups and more writers. [cloudwatch_disable](/configuration/rack-parameters/aws/cloudwatch_disable) covers more groups and fewer writers. Each one stops something the other leaves running, and setting one does not imply the other.

| | `app_cloudwatch_disable=true` | `cloudwatch_disable=true` |
|---|---|---|
| Writers stopped | Rack controller and Fluentd | Rack controller only |
| Groups covered | Per-App groups only | Per-App groups and the Rack system group |
| Fluentd | Stops shipping App container output | Unaffected, keeps creating and filling both groups |
| `convox logs -a my-app` | Empty | Empty |
| `convox rack logs` | Unchanged | Empty |

## Default Value

The default value is `false`.

## What Changes When Enabled

| Command | Behavior with `app_cloudwatch_disable=true` |
|---------|----------------------------------------------|
| `convox logs -a my-app` | Returns empty. The whole-App view is served from CloudWatch. |
| `convox logs -a my-app -s my-service` | Unchanged. The per-Service view reads Pod logs directly. |
| `convox rack logs` | Unchanged. The Rack system group is exempt. |
| `convox builds logs` | Unchanged. A running Build streams from the build Pod, and a finished Build reads its stored log object. |
| `convox ps` | Unchanged. |
| `convox deploy` and `convox releases promote` tail output | Process state lines still print. Kubernetes event lines and deploy state lines stop. |

## Interaction with fluentd_disable

With `app_cloudwatch_disable=true`, App container output has already stopped reaching CloudWatch, so [fluentd_disable](/configuration/rack-parameters/aws/fluentd_disable) changes only what Fluentd contributes to the Rack system group and whether syslog forwarding runs at all.

| `app_cloudwatch_disable` | `fluentd_disable` | Per-App groups `/convox/<rack>/<app>` | Rack system group `/convox/<rack>/system` |
|--------------------------|-------------------|---------------------------------------|-------------------------------------------|
| `false` | `false` | Fluentd container output, plus Rack Kubernetes events and deploy state | Fluentd Rack system Pod output and Nginx access logs, plus Rack events |
| `true` | `false` | Nothing. Neither writer sends App records. | Unchanged |
| `false` | `true` | Rack Kubernetes events and deploy state only, no container output | Rack events only, no Nginx access logs |
| `true` | `true` | Nothing | Rack events only, no Nginx access logs |

Adding `cloudwatch_disable=true` to any of these rows stops the Rack controller on both groups and makes `convox rack logs` return empty. Fluentd keeps writing Rack system Pod output and Nginx access logs into `/convox/<rack>/system` unless `fluentd_disable` is also `true`.

## Syslog Forwarding

Turning this parameter on does not stop [syslog](/configuration/rack-parameters/aws/syslog) forwarding. Fluentd keeps its syslog store on Rack system records, and App records reach a catch-all that carries a syslog copy whenever the `syslog` parameter is set. App output leaves CloudWatch and still leaves the Rack. With `syslog` unset, App records are discarded.

## Warnings

Three warnings can print on stderr when you set logging parameters on an AWS Rack. None of them blocks the change; the parameters are applied either way. "Effective" below means the value set in the same command, falling back to the value already stored on the Rack.

Setting `app_cloudwatch_disable=true` when the effective `cloudwatch_disable` is `true`, or setting `cloudwatch_disable=true` when the effective `app_cloudwatch_disable` is `true`:

```text
WARNING: cloudwatch_disable also covers the rack system log group, so 'convox rack logs' stays empty while it is set. Set cloudwatch_disable=false to get the rack view back.
```

Setting `fluentd_disable=true` when neither the effective `cloudwatch_disable` nor the effective `app_cloudwatch_disable` is `true`:

```text
WARNING: disabling fluentd stops application logs, but the rack still creates an empty CloudWatch log group per app (/convox/<rack>/<app>). Set app_cloudwatch_disable=true to prevent them while keeping 'convox rack logs', or cloudwatch_disable=true to stop all rack CloudWatch.
```

Setting `cloudwatch_disable=true` when neither the effective `fluentd_disable` nor the effective `app_cloudwatch_disable` is `true`:

```text
WARNING: cloudwatch_disable stops Convox rack-side CloudWatch writes and makes 'convox logs' return empty, but fluentd is still enabled and will keep shipping application logs to CloudWatch and creating /convox/<rack>/<app> groups. To stop CloudWatch entirely, also set fluentd_disable=true.
```

## Setting the Parameter

To stop the per-App CloudWatch groups while keeping the Rack view:

```bash
$ convox rack params set app_cloudwatch_disable=true -r rackName
Updating parameters... OK
```

To take a Rack off CloudWatch entirely, set `cloudwatch_disable=true` and `fluentd_disable=true` instead. `app_cloudwatch_disable` does not cover the Rack system group.

Enabling or disabling this parameter re-renders the Fluentd configuration, so the Fluentd DaemonSet rolls once while the apply runs.

## Existing Log Groups

Turning the parameter on deletes nothing. Groups that already exist keep their contents, and they keep whatever retention policy was last applied to them. The Rack stops calling the CloudWatch retention APIs for App groups, so retention is frozen at that value until the parameter is turned off again.

App lines produced while the parameter is on are dropped rather than buffered. They are not replayed when you turn the parameter off. Turning it off restores both writers and both reads immediately, and each App group is recreated on the next write to it.

## Additional Information

This parameter is available on AWS Racks only and requires Rack version `3.25.5` or later. Setting it also requires a `convox` CLI at `3.25.5` or newer; an older CLI rejects the name as an unknown parameter, so run [`sudo convox update`](/reference/cli/update) first.

Downgrading below `3.25.5` removes the parameter. Parameter reconciliation deletes it from the stored values before the apply runs and prints `NOTICE: removing parameters not supported by version <version>: app_cloudwatch_disable` on stderr. Both writers resume and each App group is recreated on the next write. Lines dropped while the parameter was on are not replayed. Upgrading back to `3.25.5` or later does not restore the value, because the downgrade deleted it, so set it again after the upgrade.

- **Validation:** must be `true` or `false`. Any other value is rejected.
- The parameter is not clearable. Passing an empty value is rejected with `param 'app_cloudwatch_disable' requires an explicit value (omit to keep current)`. Set it to `false` to turn it off.
- `convox rack params` lists stored values only, so `app_cloudwatch_disable` does not appear in its output until you set it.
- The parameter belongs to the `logging` parameter group, so `convox rack params -g logging` surfaces it once it is set.
- The `awsLogs` App setting (`cwRetention` and `disableRetention`) is a no-op while this parameter is on. Promoting a Release does not apply the retention policy, and existing groups keep the retention they already had. See [App Settings](/configuration/app-settings).
- The `--filter` flag on `convox logs` is applied by CloudWatch, so it has nothing to match on the whole-App view while this parameter is on. It is not applied on the `--service` path.
- [access_log_retention_in_days](/configuration/rack-parameters/aws/access_log_retention_in_days) sets the retention Fluentd applies to the Nginx access log stream, which lives in the Rack system group and is unaffected by this parameter.
- [eks_log_types](/configuration/rack-parameters/aws/eks_log_types) controls EKS control plane logging, which uses a separate log group and is unrelated to this parameter.

## See Also

- [cloudwatch_disable](/configuration/rack-parameters/aws/cloudwatch_disable) to stop the Rack's own CloudWatch writes and reads across both groups
- [fluentd_disable](/configuration/rack-parameters/aws/fluentd_disable) to stop Fluentd shipping container output to CloudWatch
- [syslog](/configuration/rack-parameters/aws/syslog) for forwarding logs to an external syslog endpoint
- [access_log_retention_in_days](/configuration/rack-parameters/aws/access_log_retention_in_days) for Nginx access log retention
- [Logging](/configuration/logging) for an overview of Convox logging
- [App Settings](/configuration/app-settings) for the per-App `awsLogs` retention settings
