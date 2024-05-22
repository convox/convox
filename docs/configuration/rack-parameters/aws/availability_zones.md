---
title: "availability_zones"
draft: false
slug: availability_zones
url: /configuration/rack-parameters/aws/availability_zones
---

# availability_zones

## Description
The `availability_zones` parameter specifies a list of Availability Zone (AZ) names (minimum 3) to override the random automatic selection by AWS. By defining specific AZs, you can ensure that your resources are distributed across the desired zones for better availability and fault tolerance.

## Default Value
The default value for `availability_zones` is an empty string. When set to an empty string, Convox will randomly choose AZs from your chosen region.

## Use Cases
- **Controlled Resource Distribution**: By specifying AZs, you can control how your resources are distributed across the AWS region, which can be important for compliance or operational reasons.
- **High Availability**: Ensuring that your applications and services are spread across multiple AZs enhances fault tolerance and availability.

## Setting Parameters
To set the `availability_zones` parameter, use the following command:
```html
$ convox rack params set availability_zones=us-east-1a,us-east-1b,us-east-1c -r rackName
Setting parameters... OK
```
This command sets the availability zones to `us-east-1a`, `us-east-1b`, and `us-east-1c`.

## Additional Information
Specifying AZs can help you optimize resource placement based on your application's requirements. For example, you might choose AZs based on their proximity to your user base or other AWS services. Ensure that the chosen AZs are available in your AWS region and that they meet your redundancy and latency requirements.
