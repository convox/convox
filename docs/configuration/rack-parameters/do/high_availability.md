---
title: "high_availability"
slug: high_availability
url: /configuration/rack-parameters/do/high_availability
---

# high_availability

## Description
The `high_availability` parameter controls whether your Convox rack on Digital Ocean runs in high availability mode with multiple nodes for redundancy.

## Default Value
The default value for `high_availability` is `true`.

## Use Cases
- **Production Environments**: Keep enabled (default) for production workloads that require fault tolerance and zero-downtime deployments.
- **Development/Testing**: Set to `false` for non-production environments to reduce costs by running fewer nodes.

## Setting Parameters
To set the `high_availability` parameter, use the following command:
```bash
$ convox rack params set high_availability=false -r rackName
Setting parameters... OK
```

## Additional Information
When `high_availability` is enabled, the rack runs multiple nodes across the cluster to ensure that workloads can be rescheduled if a node fails. Disabling this parameter reduces the rack to a minimal node configuration, which lowers costs but removes redundancy.
