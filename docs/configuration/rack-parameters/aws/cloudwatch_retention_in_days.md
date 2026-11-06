---
title: "cloudwatch_retention_in_days"
description: "The cloudwatch_retention_in_days AWS rack parameter sets how long CloudWatch keeps the Rack, App, and EKS control plane log groups, and is unset by default."
slug: cloudwatch_retention_in_days
url: /configuration/rack-parameters/aws/cloudwatch_retention_in_days
---

# cloudwatch_retention_in_days

## Description

The `cloudwatch_retention_in_days` parameter sets how long CloudWatch keeps the log groups a Rack owns. One value covers the Rack system group, every App log group, and the EKS control plane group.

| Value | Behavior |
|-------|----------|
| Unset (the default) | Convox does not manage retention on a group that already exists. Every covered log group keeps the value it already has, including one set by hand in the CloudWatch console. A log group Convox creates starts at 7 days. |
| `Never` | Convox removes the retention policy from the covered groups, so they stop expiring. CloudWatch reports this as `Never expire`. |
| A period | The covered groups keep logs for that many days. |

## Default Value

The default is unset. `convox rack params` lists stored values only, so this parameter does not appear in that output until you set it, and an absent entry means Convox is not managing retention.

## Accepted Values

`Never`, or one of the periods CloudWatch accepts, in days:

1, 3, 5, 7, 14, 30, 60, 90, 120, 150, 180, 365, 400, 545, 731, 1096, 1827, 2192, 2557, 2922, 3288, 3653

Any other value is rejected with the list, so `31` is refused rather than becoming `60`:

```text
param 'cloudwatch_retention_in_days' must be Never or one of the periods AWS allows: 1, 3, 5, 7, 14, 30, 60, 90, 120, 150, 180, 365, 400, 545, 731, 1096, 1827, 2192, 2557, 2922, 3288, 3653
```

## Log Groups Covered

- The Rack system group `/convox/<rack>/system`
- Every App log group `/convox/<rack>/<app>`
- The EKS control plane group `/aws/eks/<rack>/cluster`. AWS creates it when [eks_log_types](/configuration/rack-parameters/aws/eks_log_types) is first set and does not delete it when the parameter is cleared, so the group stays covered after control plane logging is turned off

## Setting a Period Deletes Log Data

Setting this parameter, or lowering it, deletes log data. CloudWatch applies a retention policy to the data already in a log group, so everything older than the new window is gone within hours. Raising the value later does not bring it back.

It also overwrites a retention policy you set by hand in the CloudWatch console, on any group it covers. While the parameter is unset, a policy you set by hand is left alone.

Read the retention the groups carry today before you set a value:

```bash
$ aws logs describe-log-groups --log-group-name-prefix /convox/<rack>/ --query 'logGroups[].[logGroupName,retentionInDays]' --output table
```

A `retentionInDays` of `None` means that group never expires. Deploy any App that needs to keep its own retention before you set a value, as described under Per-App Overrides below.

## Per-App Overrides

An App whose current Release sets `appSettings.awsLogs.cwRetention` to 1 or more, or `appSettings.awsLogs.disableRetention: true`, keeps its own value. This parameter never overwrites it.

The exemption is read from the Release the App is running, not from the `convox.yml` in your working copy. Adding the setting protects the App only once `convox deploy` promotes a Release carrying it. Deploy first, then set the parameter. Editing `convox.yml` without deploying leaves the App covered by this parameter like any other, and its log data older than the new window is gone.

An `awsLogs` block that sets neither expresses no preference, so that App's log group follows this parameter. See [App Settings](/configuration/app-settings).

## Setting the Parameter

```bash
$ convox rack params set cloudwatch_retention_in_days=30 -r rackName
Updating parameters... OK
```

To stop the covered groups expiring:

```bash
$ convox rack params set cloudwatch_retention_in_days=Never -r rackName
Updating parameters... OK
```

### Clearing the Parameter

```bash
$ convox rack params set cloudwatch_retention_in_days= -r rackName
Updating parameters... OK
```

Clearing hands retention back to you. Convox stops applying a value, every log group keeps the retention it last had, and nothing already in CloudWatch is reset. A log group Convox creates afterwards starts at 7 days. `convox rack params` then lists the parameter with an empty value, which is how a cleared parameter reads.

## When the Value Reaches a Log Group

`convox rack params set` is asynchronous, so the value is not applied when the command returns. Convox applies it once the Rack update finishes. A log group that appears after that, an App deployed later for example, is covered within an hour rather than the instant it appears.

## Interaction with cloudwatch_disable and app_cloudwatch_disable

Neither [cloudwatch_disable](/configuration/rack-parameters/aws/cloudwatch_disable) nor [app_cloudwatch_disable](/configuration/rack-parameters/aws/app_cloudwatch_disable) turns this parameter off. They stop Convox writing to and reading from the log groups. The groups themselves stay, and Fluentd can still create them, so `cloudwatch_retention_in_days` keeps applying to them.

The `awsLogs` App setting behaves the other way: it is a no-op while either parameter is on. An App that sets `awsLogs` is still exempt from this parameter on such a Rack, and its own value is not applied either, so its log group keeps the retention it already has.

## Additional Information

This parameter is available on AWS Racks only and requires Rack version `3.25.7` or later. Setting it also requires a `convox` CLI at `3.25.7` or newer; an older CLI rejects the name as an unknown parameter, so run [`sudo convox update`](/reference/cli/update) first.

Downgrading below `3.25.7` removes the parameter. Parameter reconciliation deletes it from the stored values before the apply runs and prints `NOTICE: removing parameters not supported by version <version>: cloudwatch_retention_in_days` on stderr. Convox stops applying retention and every log group keeps the policy it already has, because those policies live in CloudWatch rather than in the Rack's Terraform state. To put a group back to never expire, set `cloudwatch_retention_in_days=Never` before downgrading, or remove the policy in the CloudWatch console. On a Rack managed through the Console the value stays in the Console's stored parameters, so `convox rack params` keeps listing it while the Rack is on the older version even though nothing is applying it, and it is applied again when you upgrade back to `3.25.7` or later. On a self-managed Rack the value is removed from the stored parameters, so set it again after the upgrade.

- **Validation:** must be `Never` or one of the periods listed above. `Never` is case-sensitive, matching [karpenter_node_expiry](/configuration/rack-parameters/aws/karpenter_node_expiry). `0` is rejected; clear the parameter to stop managing retention.
- **Clearable:** setting an empty value stops Convox managing retention and leaves every policy where it is.
- `convox rack params` lists stored values only, so `cloudwatch_retention_in_days` does not appear in its output until you set it.
- The parameter belongs to the `logging` parameter group, so `convox rack params -g logging` surfaces it once it is set.

## See Also

- [cloudwatch_disable](/configuration/rack-parameters/aws/cloudwatch_disable) to stop the Rack's own CloudWatch writes and reads across both groups
- [app_cloudwatch_disable](/configuration/rack-parameters/aws/app_cloudwatch_disable) to stop both CloudWatch writers for the per-App groups
- [eks_log_types](/configuration/rack-parameters/aws/eks_log_types) for the EKS control plane log group
- [access_log_retention_in_days](/configuration/rack-parameters/aws/access_log_retention_in_days), which is stored but sets no retention on any log group
- [fluentd_disable](/configuration/rack-parameters/aws/fluentd_disable) to stop Fluentd shipping container output to CloudWatch
- [App Settings](/configuration/app-settings) for the per-App `awsLogs` override
- [Logging](/configuration/logging) for an overview of Convox logging
