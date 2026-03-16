---
title: "node_disk"
slug: node_disk
url: /configuration/rack-parameters/gcp/node_disk
---

# node_disk

## Description
The `node_disk` parameter specifies the size of the root disk (in GB) for each node in your Convox rack on GCP.

## Default Value
The default value for `node_disk` is `100`.

## Use Cases
- **Storage-Intensive Workloads**: Increase disk size for applications that require more local storage for builds, caches, or temporary data.
- **Cost Management**: Use the default size when local storage requirements are minimal.

## Setting Parameters
To set the `node_disk` parameter, use the following command:
```bash
$ convox rack params set node_disk=200 -r rackName
Setting parameters... OK
```

## Additional Information
The disk size is specified in gigabytes (GB). Larger disks provide more space for container images, build caches, and application data stored locally on nodes.
