---
title: "region"
draft: false
slug: region
url: /configuration/rack-parameters/gcp/region
---

# region

## Description
The `region` parameter specifies the GCP region where your Convox rack will be deployed. Choosing the appropriate region can impact latency, availability, and cost.

## Default Value
The default value for `region` is `us-east1`.

## Use Cases
- **Latency Optimization**: Select a region that is geographically close to your users to reduce latency.
- **Compliance**: Choose a region that meets data residency and compliance requirements.
- **Cost Management**: Different regions may have different pricing, so selecting the right region can help manage costs.

## Setting Parameters
To set the `region` parameter, use the following command:
```html
$ convox rack params set region=us-east1 -r rackName
Setting parameters... OK
```
This command sets the `region` parameter to the specified value.

## Additional Information
Selecting the appropriate region is crucial for optimizing the performance and compliance of your applications. For more information on GCP regions, refer to the [GCP documentation on regions and zones](https://cloud.google.com/about/locations).
