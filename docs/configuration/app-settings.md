---
title: "App Settings"
description: "App settings are per-App configuration in convox.yml, currently the awsLogs section, which overrides the Rack-wide CloudWatch log retention for one App."
slug: app-settings
url: /configuration/app-settings
---
# App Settings

App settings are configuration parameters specific to a particular [App](/reference/primitives/app) within a Convox rack. They provide a flexible way to customize the behavior and functionality of an app without affecting the overall rack configuration. This is especially useful for adapting apps to different environments like development, staging, and production.

```yaml
appSettings:
  awsLogs:
    cwRetention: 30
    disableRetention: false
```

Currently, the `appSettings` section supports the `awsLogs` parameter.

## AWS Logs

The `awsLogs` section allows you to configure the retention time and policy for the AWS CloudWatch log group associated with your app.

| Attribute     | Type       | Default             | Description                                                                                                                                |
| ------------- | ---------- | ------------------- | ------------------------------------------------------------------------------------------------------------------------------------------ |
| **cwRetention** | int | unset | How many days CloudWatch keeps this App's log group. Unset means the App expresses no preference. |
| **disableRetention** | boolean | `false` | `true` removes the retention policy, so this App's log group never expires. |

Important Notes:

- `cwRetention` must be one of the periods CloudWatch accepts: 1, 3, 5, 7, 14, 30, 60, 90, 120, 150, 180, 365, 400, 545, 731, 1096, 1827, 2192, 2557, 2922, 3288, 3653. Any other number is rounded up to the next one on the list, so `31` becomes `60`. A value above 3653 becomes 3653.
- To completely disable retention and retain logs indefinitely, set the `disableRetention` parameter to `true`.
- An `awsLogs` block that sets neither a `cwRetention` of 1 or more nor `disableRetention: true` expresses no preference. From Rack version `3.25.7`, the App's log group then follows [cloudwatch_retention_in_days](/configuration/rack-parameters/aws/cloudwatch_retention_in_days), or keeps the retention it already has when that parameter is unset. A log group Convox creates starts at 7 days.
- On Rack version `3.25.6` and earlier the same block set the App's log group to 1 day, shorter than the 7 days an App with no `awsLogs` block at all keeps.
- `convox.yml` parsing ignores keys it does not recognize, so a misspelled key under `awsLogs` is dropped without an error and the block is read as setting nothing. From Rack version `3.25.7` the App's log group then follows the Rack parameter; on `3.25.6` and earlier it fell to 1 day.
- The `awsLogs` settings have no effect on a Rack with either [cloudwatch_disable](/configuration/rack-parameters/aws/cloudwatch_disable) or [app_cloudwatch_disable](/configuration/rack-parameters/aws/app_cloudwatch_disable) set to `true`. The retention policy is not applied, and existing log groups keep whatever retention was last set.

