---
title: "compartment_ocid"
description: "The compartment_ocid Oracle Cloud rack parameter sets the OCI compartment where rack resources are created, defaulting to the tenancy root compartment."
slug: compartment_ocid
url: /configuration/rack-parameters/oci/compartment_ocid
---

# compartment_ocid

## Description
The `compartment_ocid` parameter specifies the OCID of the OCI compartment where the rack's compute, network, and container engine resources are created. OCIR repositories for the rack's images also live in this compartment.

## Default Value
The default value for `compartment_ocid` is unset, which resolves to your tenancy's root compartment.

## Use Cases
- **Resource Organization**: Group a rack's resources into a dedicated compartment alongside other workloads managed by the same team.
- **Access Control**: Scope IAM policies to a compartment narrower than the tenancy root, limiting what the installing user needs tenancy-wide access to.
- **Cost Tracking**: Attribute OCI spend to a specific compartment for chargeback or budgeting.

## Setting Parameters
`compartment_ocid` can only be set at install time:
```bash
$ convox rack install oci myrack compartment_ocid=ocid1.compartment.oc1..aaaaaaaaexampleuniqueid
```

## Additional Information
Rack resources are created directly in this compartment, so changing it after install is not supported; installing into a new compartment requires a new rack. The user or API key installing the rack still needs permission to manage IAM resources at the tenancy level, since the rack creates its own IAM user, group, policy, and auth token for pushing images to OCIR.
