---
title: "high_availability"
description: "The high_availability Digital Ocean rack parameter, set at install, runs the Rack across multiple nodes when true or a minimal configuration when false."
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
The `high_availability` parameter can only be set when the Rack is installed:
```bash
$ convox rack install do <name> high_availability=false
```
It cannot be changed afterwards. `convox rack params set` rejects it with `param 'high_availability' can only be set during rack installation`.

## Additional Information
When `high_availability` is enabled, the rack runs multiple nodes across the cluster to ensure that workloads can be rescheduled if a node fails. Disabling this parameter reduces the rack to a minimal node configuration, which lowers costs but removes redundancy.

`high_availability` also sets the number of Elasticsearch Pods that store the Rack's logs: two, or one when it is `false`. Each Pod has its own volume, sized by [elasticsearch_disk](/configuration/rack-parameters/do/elasticsearch_disk).
