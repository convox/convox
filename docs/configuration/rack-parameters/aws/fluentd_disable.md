---
title: "fluentd_disable"
description: "The fluentd_disable AWS rack parameter disables the installation of Fluentd, the component Convox uses to send logs to CloudWatch, defaulting to false."
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
```bash
$ convox rack params set fluentd_disable=true -r rackName
Updating parameters... OK
```
This command disables the installation of Fluentd in your rack.

## Additional Information
Disabling Fluentd can be beneficial if you have a different logging infrastructure in place or if you want to reduce the overhead of running additional services. Without Fluentd, you will need to ensure that your logs are still being captured and managed effectively by your alternative logging solution.

Fluentd forwards application container output to CloudWatch. Disabling it does not take the Rack off CloudWatch: the Rack still creates a log group per App (`/convox/<rack>/<app>`) and writes Kubernetes events and deploy state into it. Two parameters stop that, and they differ in what else goes with it.

| Parameter | Per-App groups `/convox/<rack>/<app>` | `convox rack logs` |
|-----------|---------------------------------------|--------------------|
| [app_cloudwatch_disable](/configuration/rack-parameters/aws/app_cloudwatch_disable) | Not created, not written | Unchanged |
| [cloudwatch_disable](/configuration/rack-parameters/aws/cloudwatch_disable) | Not created, not written | Returns empty |

Set `app_cloudwatch_disable=true` to keep the Rack view. Set `cloudwatch_disable=true` when you want the Rack system group closed off as well. Setting `fluentd_disable=true` with neither of them prints a warning naming both. Make sure to configure your logging system to maintain centralized log management if you choose to disable Fluentd.

## See Also
- [app_cloudwatch_disable](/configuration/rack-parameters/aws/app_cloudwatch_disable) to stop the per-App log groups while keeping `convox rack logs`
- [cloudwatch_disable](/configuration/rack-parameters/aws/cloudwatch_disable) to stop the Rack's own CloudWatch writes and reads
- [fluentd_memory](/configuration/rack-parameters/aws/fluentd_memory) to adjust Fluentd memory allocation without disabling it
- [syslog](/configuration/rack-parameters/aws/syslog) for forwarding logs to an external syslog endpoint
- [Logging](/configuration/logging) for an overview of Convox logging