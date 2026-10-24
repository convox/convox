---
title: "node_disk"
description: "The node_disk Azure rack parameter sets the OS disk size in GB for the default AKS node pool and the default for additional pools, defaulting to 30."
slug: node_disk
url: /configuration/rack-parameters/azure/node_disk
---

# node_disk

## Description
The `node_disk` parameter sets the OS disk size in GB for nodes in the default AKS node pool. This value is also used as the default disk size for additional node pools when no `disk` value is specified.

## Default Value
The default value for `node_disk` is `30`.

## Use Cases
- **Storage-Intensive Workloads**: Increase the disk size to accommodate applications that require more local storage for caching, logs, or temporary files.
- **Container Image Storage**: Allocate additional disk space when your deployments use large container images that need to be pulled and stored on each node.

## Setting Parameters
To set the `node_disk` parameter, use the following command:
```bash
$ convox rack params set node_disk=50 -r rackName
Updating parameters... OK
```
This command sets the OS disk size for nodes to `50` GB.

Changing `node_disk` replaces every node of the default node pool through a temporary node pool named `defaulttemp`. Pods on the default pool are drained and rescheduled twice, onto the temporary pool and then back onto the new nodes. Each node pool in [`additional_node_groups_config`](/configuration/rack-parameters/azure/additional_node_groups_config) and [`additional_build_groups_config`](/configuration/rack-parameters/azure/additional_build_groups_config) that does not set its own `disk` takes its disk size from `node_disk`, so the change rotates those pools the same way, each through its own temporary pool.
