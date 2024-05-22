---
title: "access_log_retention_in_days"
draft: false
slug: access_log_retention_in_days
url: /configuration/rack-parameters/aws/access_log_retention_in_days
---

# access_log_retention_in_days

## Description
The `access_log_retention_in_days` parameter specifies the retention period for Nginx access logs stored in CloudWatch Logs. The log group name will be `/convox/<rack-name>/system`, and the stream name will be `/nginx-access-logs`.

## Default Value
The default value for `access_log_retention_in_days` is `7`.

## Use Cases
- **Regulatory Compliance**: Ensuring logs are retained for a specific period to comply with regulatory requirements.
- **Audit and Monitoring**: Keeping logs available for auditing purposes or for detailed monitoring and troubleshooting over a defined period.

## Managing Parameters

### Viewing Current Parameters
```html
$ convox rack params -r rackName
access_log_retention_in_days  7
```

### Setting Parameters
To set the `access_log_retention_in_days` parameter, use the following command:
```html
$ convox rack params set access_log_retention_in_days=30 -r rackName
Setting parameters... OK
```
This command sets the access log retention period to 30 days.

## Additional Information
Adjusting the log retention period allows you to manage the volume of stored logs and control storage costs in CloudWatch. Consider your organization's logging policies and compliance requirements when setting this parameter. Remember, longer retention periods can be useful for historical analysis but will incur higher storage costs.
