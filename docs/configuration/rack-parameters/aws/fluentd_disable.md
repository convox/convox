---
title: "fluentd_disable"
draft: false
slug: fluentd_disable
url: /configuration/rack-parameters/aws/fluentd_disable
---

# fluentd_disable

## Description
The `fluentd_disable` parameter disables the installation of Fluentd in the rack. Fluentd is used by Convox to send logs to CloudWatch, providing a centralized logging solution for your applications and infrastructure.

## Default Value
The default value for `fluentd_disable` is `false`.

## Use Cases
- **Custom Logging Solutions**: Disable Fluentd if you are using an alternative logging solution.
- **Resource Optimization**: Reduce resource usage by disabling unnecessary components if Fluentd is not required.

## Setting Parameters
To disable Fluentd, use the following command:
```html
$ convox rack params set fluentd_disable=true -r rackName
Setting parameters... OK
```
This command disables the installation of Fluentd in your rack.

## Additional Information
Disabling Fluentd can be beneficial if you have a different logging infrastructure in place or if you want to reduce the overhead of running additional services. Without Fluentd, you will need to ensure that your logs are still being captured and managed effectively by your alternative logging solution.

Fluentd is an essential component in Convox for forwarding logs to CloudWatch. Make sure to configure your logging system to maintain centralized log management if you choose to disable Fluentd.