---
title: "App Settings"
draft: false
slug: App Settings
url: /configuration/app-settings
---
# App Settings

App settings are configuration parameters specific to a particular App [App](/reference/primitives/app). within a Convox rack. They provide a flexible way to customize the behavior and functionality of an app without affecting the overall rack configuration. This is especially useful for adapting apps to different environments like development, staging, and production.

```html
    appSettings:
      awsLogs:
        cwRetention: 31
        disableRetention: false
```

Currently, the appSettings section supports the awsLogs parameter, but additional parameters may be added in the future.

## Aws Logs

The `awsLogs` section allows you to configure the retention time and policy for the AWS CloudWatch log group associated with your app.


| Attribute     | Type       | Default             | Description                                                                                                                                |
| ------------- | ---------- | ------------------- | ------------------------------------------------------------------------------------------------------------------------------------------ |
| **cwRetention**       | int    | 7               | Specifies the retention period in days for CloudWatch logs.
| **disableRetention**       | boolean    | false               | Indicates whether to disable retention and retain logs indefinitely

Important Notes:

- The cwRetention parameter accepts values from the predefined list of CloudWatch retention policy options. If an invalid value is specified, it will automatically be adjusted to the nearest valid higher value.
- Retention periods exceeding 10 years are not supported.
- To completely disable retention and retain logs indefinitely, set the disableRetention parameter to true.


