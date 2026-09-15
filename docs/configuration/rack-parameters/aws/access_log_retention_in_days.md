---
title: "access_log_retention_in_days"
description: "The access_log_retention_in_days AWS rack parameter is passed to Fluentd but sets no retention on any CloudWatch log group; cloudwatch_retention_in_days controls Rack log retention."
slug: access_log_retention_in_days
url: /configuration/rack-parameters/aws/access_log_retention_in_days
---

# access_log_retention_in_days

## Description
The `access_log_retention_in_days` parameter sets `retention_in_days` on the Fluentd output that ships Nginx access logs to CloudWatch Logs. Those logs go to the Rack system group `/convox/<rack>/system`, in the stream `/nginx-access-logs`.

The value is not applied to that log group. Fluentd writes a retention policy only for a log group it creates itself, the Rack creates `/convox/<rack>/system` when it writes its first event, and Fluentd's IAM role does not grant `logs:PutRetentionPolicy`. Use [cloudwatch_retention_in_days](/configuration/rack-parameters/aws/cloudwatch_retention_in_days) to control how long CloudWatch keeps the Rack system group.

## Default Value
The default value for `access_log_retention_in_days` is `7`.

## Setting Parameters
To set the `access_log_retention_in_days` parameter, use the following command:
```bash
$ convox rack params set access_log_retention_in_days=30 -r rackName
Updating parameters... OK
```
The value is stored and rendered into the Fluentd configuration. Changing it rolls the Fluentd DaemonSet once while the apply runs. It does not change the retention on any log group.

## Additional Information
The parameter is still accepted and still stored. It is not deprecated and it is not replaced by another parameter, and setting it is harmless. It has no effect on how long CloudWatch keeps anything.

## See Also
- [cloudwatch_retention_in_days](/configuration/rack-parameters/aws/cloudwatch_retention_in_days) for how long CloudWatch keeps the Rack, App, and EKS control plane log groups
- [App Settings](/configuration/app-settings) for the per-App `awsLogs` override
- [Logging](/configuration/logging) for an overview of Convox logging
