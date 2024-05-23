---
title: "high_availability"
draft: false
slug: high_availability
url: /configuration/rack-parameters/aws/high_availability
---

# high_availability

## Description
The `high_availability` parameter determines whether to create a high availability (HA) cluster. Setting this parameter to `false` will create a cluster with less redundant resources, which can help with cost optimization.

## Default Value
The default value for `high_availability` is `true`.

## Use Cases
- **Cost Optimization**: Reduce infrastructure costs by creating a less redundant cluster.
- **Resource Management**: Allocate fewer resources for environments where high availability is not critical.

## Setting Parameters
The `high availability` parameter must be configured at rack installation. Example:
| Key                    | Value                                         |
|------------------------|-----------------------------------------------|
| `high availability`  | `true` |

&nbsp;

## Additional Information
High availability clusters provide increased resilience and uptime by using redundant resources. Disabling high availability can significantly reduce costs, making it suitable for non-production environments, development clusters, or any scenario where uptime is not critical.

Before setting `high_availability` to `false`, consider the potential impact on service reliability and downtime.

Maintaining high availability is crucial for production environments that require continuous operation and minimal downtime.
