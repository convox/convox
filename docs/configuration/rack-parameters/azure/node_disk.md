---
title: "node_disk"
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
Setting parameters... OK
```
This command sets the OS disk size for nodes to `50` GB.
