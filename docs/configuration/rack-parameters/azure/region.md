---
title: "region"
draft: false
slug: region
url: /configuration/rack-parameters/azure/region
---

# region

## Description
The `region` parameter specifies the Azure region where your Convox rack will be deployed. Choosing the appropriate region can impact latency, availability, and cost.

## Default Value
The default value for `region` is `eastus`.

## Use Cases
- **Latency Optimization**: Select a region that is geographically close to your users to reduce latency.
- **Compliance**: Choose a region that meets data residency and compliance requirements.
- **Cost Management**: Different regions may have different pricing, so selecting the right region can help manage costs.

## Setting Parameters
To set the `region` parameter, use the following command:
```html
$ convox rack params set region=eastus -r rackName
Setting parameters... OK
```
This command sets the `region` parameter to the specified value.

## Additional Information
Selecting the appropriate region is crucial for optimizing the performance and compliance of your applications. For more information on Azure regions, refer to the [Azure documentation on regions and availability zones](https://azure.microsoft.com/en-us/global-infrastructure/geographies/).
