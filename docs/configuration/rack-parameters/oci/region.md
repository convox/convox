---
title: "region"
description: "The region Oracle Cloud rack parameter sets the OCI region where the Convox rack is deployed, affecting latency, availability, and cost, defaulting to us-ashburn-1."
slug: region
url: /configuration/rack-parameters/oci/region
---

# region

## Description
The `region` parameter specifies the [OCI region](https://docs.oracle.com/en-us/iaas/Content/General/Concepts/regions.htm) where your Convox rack will be deployed. Choosing the appropriate region can impact latency, availability, and cost.

## Default Value
The default value for `region` is `us-ashburn-1`.

## Use Cases
- **Latency Optimization**: Select a region that is geographically close to your users to reduce latency.
- **Compliance**: Choose a region that meets data residency and compliance requirements.
- **Cost Management**: Different regions may have different pricing, so selecting the right region can help manage costs.

## Setting Parameters
`region` can only be set at install time:
```bash
$ convox rack install oci myrack region=us-ashburn-1
```

## Additional Information
Selecting the appropriate region is crucial for optimizing the performance and compliance of your applications. For more information on OCI regions, refer to the [OCI documentation on regions and availability domains](https://docs.oracle.com/en-us/iaas/Content/General/Concepts/regions.htm).
