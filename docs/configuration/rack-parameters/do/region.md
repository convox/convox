---
title: "region"
draft: false
slug: region
url: /configuration/rack-parameters/do/region
---

# region

## Description
The `region` parameter specifies the [Digital Ocean region](https://slugs.do-api.dev/) where your Convox rack will be deployed. Choosing the appropriate region can impact latency, availability, and cost.

## Default Value
The default value for `region` is `nyc3`.

## Use Cases
- **Latency Optimization**: Select a region that is geographically close to your users to reduce latency.
- **Compliance**: Choose a region that meets data residency and compliance requirements.
- **Cost Management**: Different regions may have different pricing, so selecting the right region can help manage costs.

## Setting Parameters
To set the `region` parameter, use the following command:
```html
$ convox rack params set region=nyc3 -r rackName
Setting parameters... OK
```
This command sets the `region` parameter to the specified value.

## Additional Information
Selecting the appropriate region is crucial for optimizing the performance and compliance of your applications. For more information on Digital Ocean regions, refer to the [Digital Ocean documentation on regions](https://slugs.do-api.dev/).
