---
title: "gpu_tag_enable"
draft: false
slug: gpu_tag_enable
url: /configuration/rack-parameters/aws/gpu_tag_enable
---

# gpu_tag_enable

## Description
The `gpu_tag_enable` parameter enables GPU tagging for your instances. This allows you to tag GPU resources, which can be useful for tracking and managing GPU usage across your AWS environment.

## Default Value
The default value for `gpu_tag_enable` is `false`.

## Use Cases
- **Resource Tracking**: Tagging GPU resources to keep track of GPU usage and costs.
- **Operational Management**: Simplify the management and organization of GPU instances within your AWS infrastructure.

## Setting Parameters
To enable GPU tagging, use the following command:
```html
$ convox rack params set gpu_tag_enable=true -r rackName
Setting parameters... OK
```
This command enables GPU tagging for your instances.

## Additional Information
Enabling GPU tagging helps you manage and monitor GPU resources more effectively by allowing you to assign custom tags to GPU instances. Note that GPU tagging is not supported in all AWS regions, so ensure that your region supports this feature before enabling it. For more information on GPU tagging and supported regions, refer to the [AWS documentation](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/Using_Tags.html).

Proper tagging of GPU resources can help with cost allocation, operational management, and resource optimization across your cloud infrastructure.
