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
The `availability_zones` parameter must be configured at rack installation. Example:
| Key                    | Value                                         |
|------------------------|-----------------------------------------------|
| `availability_zones`  | `east-1a,us-east-1b,us-east-1c` |

&nbsp;

## Additional Information
Specifying AZs can help you optimize resource placement based on your application's requirements. For example, you might choose AZs based on their proximity to your user base or other AWS services. Ensure that the chosen AZs are available in your AWS region and that they meet your redundancy and latency requirements.
