---
title: "cloudwatch_disable"
description: "The cloudwatch_disable AWS rack parameter stops the Rack from creating, writing, and reading its own CloudWatch log groups, defaulting to false."
slug: cloudwatch_disable
url: /configuration/rack-parameters/aws/cloudwatch_disable
---

# cloudwatch_disable

## Description

The `cloudwatch_disable` parameter stops the Rack from creating, writing, and reading its own CloudWatch log groups. Those are the per-App group `/convox/<rack>/<app>` and the Rack system group `/convox/<rack>/system`.

While the parameter is on, the Rack does not create either group, does not write Kubernetes events, deploy state transitions, or AWS resource provisioning messages into them, does not read from them, and does not apply retention policies to them.

The parameter changes Rack behavior only. It does not stop Fluentd, which writes to the same group names. See the interaction table below.

## Default Value

The default value is `false`.

## What Changes When Enabled

| Command | Behavior with `cloudwatch_disable=true` |
|---------|------------------------------------------|
| `convox logs -a my-app` | Returns empty. The whole-App view is served from CloudWatch. |
| `convox rack logs` | Returns empty. The Rack system view is served from CloudWatch. |
| `convox logs -a my-app -s my-service` | Unchanged. The per-Service view reads Pod logs directly. |
| `convox builds logs` | Unchanged. A running Build streams from the build Pod, and a finished Build reads its stored log object. |
| `convox ps` | Unchanged. |
| `convox deploy` and `convox releases promote` tail output | Process state lines still print. Kubernetes event lines and deploy state lines stop. |

## Interaction with fluentd_disable

`cloudwatch_disable` and [fluentd_disable](/configuration/rack-parameters/aws/fluentd_disable) gate two different writers into the same CloudWatch log groups. Fluentd ships container output and Nginx access logs. The Rack controller writes Kubernetes events, deploy state, and AWS resource state.

| `cloudwatch_disable` | `fluentd_disable` | What still reaches CloudWatch | `convox logs -a my-app` | `convox rack logs` |
|----------------------|-------------------|-------------------------------|-------------------------|--------------------|
| `false` | `false` | Fluentd writes container output and Nginx access logs. The Rack writes Kubernetes events, deploy state, and AWS resource state. | Container output interleaved with Rack lines | Rack lines plus Rack system Pod output, including Nginx access logs |
| `true` | `false` | Fluentd still creates both groups and still writes container output and Nginx access logs. The Rack writes nothing. | Empty, while the group keeps filling up | Empty, while Rack system Pod output and Nginx access logs keep arriving |
| `false` | `true` | Only the Rack writes: Kubernetes events, deploy state, and AWS resource state. No container output. | Rack lines only, no container output | Rack lines, no Nginx access logs |
| `true` | `true` | Nothing. Convox neither creates nor writes either group. | Empty | Empty |

The second row is the one to watch. Setting `cloudwatch_disable=true` on its own hides the CloudWatch view without stopping the CloudWatch writes: Fluentd creates the groups if they do not exist, keeps shipping into them, and keeps incurring their storage cost, while `convox logs` and `convox rack logs` show nothing.

In all four rows, `convox logs -a my-app -s my-service`, `convox builds logs`, and `convox ps` are unchanged.

## Setting the Parameter

To take a Rack off CloudWatch entirely, which is the configuration to use when you route logs elsewhere, set both parameters in a single call:

```bash
$ convox rack params set cloudwatch_disable=true fluentd_disable=true -r rackName
Updating parameters... OK
```

To stop the Rack's own CloudWatch writes and reads while Fluentd keeps shipping container output:

```bash
$ convox rack params set cloudwatch_disable=true -r rackName
Updating parameters... OK
```

Setting either parameter to `true` while the other is not `true`, either already stored on the Rack or set in the same command, prints a warning describing what the other half still does. The batched command above sets both at once, so it does not warn. The check reads the Rack's stored values, so setting the second half later on a Rack that already has the first is silent, as is setting either parameter to `false`. The warning does not block the change; the parameters are applied either way.

## Existing Log Groups

Turning the parameter on deletes nothing. Groups that already exist keep their contents, and they keep whatever retention policy was last applied to them. The Rack stops calling the CloudWatch retention APIs, so retention is frozen at that value until the parameter is turned off again.

Rack lines produced while the parameter is on are dropped rather than buffered. They are not replayed when you turn the parameter off. Turning it off restores reads immediately, including anything Fluentd wrote to the groups while the parameter was on.

## Additional Information

This parameter is available on AWS Racks only and requires Rack version `3.25.3` or later. On an earlier Rack the command is accepted and the value is then removed by parameter reconciliation before the apply runs, so CloudWatch behavior does not change. Setting it also requires a `convox` CLI at `3.25.3` or newer; an older CLI rejects the name as an unknown parameter, so run [`sudo convox update`](/reference/cli/update) first.

- **Validation:** must be `true` or `false`. Any other value is rejected.
- `convox rack params` lists stored values only, so `cloudwatch_disable` does not appear in its output until you set it.
- The parameter belongs to the `logging` parameter group, so `convox rack params -g logging` surfaces it once it is set.
- The `awsLogs` App setting (`cwRetention` and `disableRetention`) is a no-op while this parameter is on. Promoting a Release does not apply the retention policy, and existing groups keep the retention they already had. See [App Settings](/configuration/app-settings).
- The `--filter` flag on `convox logs` is applied by CloudWatch, so it applies to the whole-App view only. It is not applied on the `--service` path.
- [eks_log_types](/configuration/rack-parameters/aws/eks_log_types) controls EKS control plane logging, which uses a separate log group and is unrelated to this parameter.
- [access_log_retention_in_days](/configuration/rack-parameters/aws/access_log_retention_in_days) sets the retention Fluentd applies to the Nginx access log stream. Fluentd applies it independently of this parameter.

## See Also

- [fluentd_disable](/configuration/rack-parameters/aws/fluentd_disable) to stop Fluentd shipping container output to CloudWatch
- [syslog](/configuration/rack-parameters/aws/syslog) for forwarding logs to an external syslog endpoint
- [access_log_retention_in_days](/configuration/rack-parameters/aws/access_log_retention_in_days) for Nginx access log retention
- [eks_log_types](/configuration/rack-parameters/aws/eks_log_types) for EKS control plane logging
- [Logging](/configuration/logging) for an overview of Convox logging
- [App Settings](/configuration/app-settings) for the per-App `awsLogs` retention settings
