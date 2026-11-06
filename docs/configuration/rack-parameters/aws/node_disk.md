---
title: "node_disk"
description: "The node_disk AWS rack parameter sets the disk size in GiB for each node in the cluster, defaulting to 20 GiB."
slug: node_disk
url: /configuration/rack-parameters/aws/node_disk
---

# node_disk

## Description
The `node_disk` parameter specifies the disk size for each node in the cluster, measured in gibibytes (GiB). This setting determines the storage capacity available for your nodes.

## Default Value
The default value for `node_disk` is `20` GiB.

## Use Cases
- **Storage Optimization**: Adjust the disk size to meet the storage requirements of your applications and workloads.
- **Performance Management**: Ensure that each node has sufficient storage to handle its tasks without running into capacity issues.

## Setting Parameters
To set the `node_disk` parameter, use the following command:
```bash
$ convox rack params set node_disk=50 -r rackName
Updating parameters... OK
```
This command sets the disk size for each node to 50 GiB.

## Additional Information
Adjusting the `node_disk` size can help you optimize the storage capacity of your cluster based on your specific needs. Larger disk sizes provide more storage for applications and data, but they may also increase costs. Ensure that the disk size you choose balances capacity, performance, and cost-effectiveness.

`node_disk` also sets the IOPS ceiling for each node's root volume. A gp3 volume takes at most 500 provisioned IOPS per GiB, so at the default 20 GiB the ceiling is 10,000 IOPS, and a higher [node_volume_iops](/configuration/rack-parameters/aws/node_volume_iops) is lowered to that ceiling rather than rejected. Each volume is measured against its own size: a node group that sets its own `disk` in [additional_node_groups_config](/configuration/rack-parameters/aws/additional_node_groups_config) gets the ceiling for that size, not the Rack's. Throughput follows from the IOPS, at a quarter of them, so see [node_volume_throughput](/configuration/rack-parameters/aws/node_volume_throughput).
