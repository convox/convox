---
title: "preemptible"
draft: false
slug: preemptible
url: /configuration/rack-parameters/gcp/preemptible
---

# preemptible

## Description
The `preemptible` parameter specifies whether to use [preemptible](https://cloud.google.com/compute/docs/instances/preemptible) instances. Preemptible instances are cost-effective but can be terminated by Google Cloud with short notice.

## Default Value
The default value for `preemptible` is `true`.

## Use Cases
- **Cost Savings**: Use preemptible instances to significantly reduce costs for workloads that can tolerate interruptions.
- **Non-critical Workloads**: Suitable for non-critical applications, batch jobs, or workloads that are designed to handle instance terminations gracefully.

## Setting Parameters
To set the `preemptible` parameter, use the following command:
```html
$ convox rack params set preemptible=true -r rackName
Setting parameters... OK
```
This command sets the `preemptible` parameter to the specified value.

## Additional Information
Preemptible instances offer significant cost savings but can be terminated by Google Cloud at any time if resources are needed elsewhere. Ensure your applications can handle interruptions gracefully if you choose to use preemptible instances. For more information, refer to the [GCP documentation on preemptible instances](https://cloud.google.com/compute/docs/instances/preemptible).
