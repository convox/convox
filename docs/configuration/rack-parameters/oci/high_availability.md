---
title: "high_availability"
description: "The high_availability Oracle Cloud rack parameter runs the rack across multiple nodes when true, or a minimal node configuration when false, defaulting to true."
slug: high_availability
url: /configuration/rack-parameters/oci/high_availability
---

# high_availability

## Description
The `high_availability` parameter controls whether your Convox rack on Oracle Cloud runs in high availability mode with multiple nodes for redundancy.

## Default Value
The default value for `high_availability` is `true`.

## Use Cases
- **Production Environments**: Keep enabled (default) for production workloads that require fault tolerance and zero-downtime deployments.
- **Development/Testing**: Set to `false` for non-production environments to reduce costs by running fewer nodes.

## Setting Parameters
To set the `high_availability` parameter, use the following command:
```bash
$ convox rack params set high_availability=false -r rackName
Updating parameters... OK
```

## Additional Information
When `high_availability` is enabled, the rack runs multiple replicas of its own system services so they can be rescheduled if a node fails. This is independent of `node_count`, which sets the size of the worker node pool; see [node_count](/configuration/rack-parameters/oci/node_count) to control how many nodes the pool runs.
