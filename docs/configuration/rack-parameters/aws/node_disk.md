---
title: "node_disk"
draft: false
slug: node_disk
url: /configuration/rack-parameters/aws/node_disk
---

# node_disk

## Description
The `node_disk` parameter specifies the disk size for each node in the cluster, measured in gigabytes (GB). This setting determines the storage capacity available for your nodes.

## Default Value
The default value for `node_disk` is `20` GB.

## Use Cases
- **Storage Optimization**: Adjust the disk size to meet the storage requirements of your applications and workloads.
- **Performance Management**: Ensure that each node has sufficient storage to handle its tasks without running into capacity issues.

## Managing Parameters

### Viewing Current Parameters
```html
$ convox rack params -r rackName
node_disk  20
node_type  t3.small
```

### Setting Parameters
To set the `node_disk` parameter, use the following command:
```html
$ convox rack params set node_disk=50 -r rackName
Setting parameters... OK
```
This command sets the disk size for each node to 50 GB.

## Additional Information
Adjusting the `node_disk` size can help you optimize the storage capacity of your cluster based on your specific needs. Larger disk sizes provide more storage for applications and data, but they may also increase costs. Ensure that the disk size you choose balances capacity, performance, and cost-effectiveness.

By configuring the `node_disk` parameter, you can ensure that your nodes have the appropriate amount of storage to support your workloads and applications effectively.
