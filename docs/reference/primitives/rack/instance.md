---
title: "Instance"
slug: instance
url: /reference/primitives/rack/instance
---
# Instance

An Instance provides capacity for running [Processes](/reference/primitives/app/process)

## List Instances
```bash
    $ convox rack instances
    ID                            STATUS   STARTED         PS  CPU    MEM    PUBLIC         PRIVATE
    ip-10-1-1-1.ec2.internal      running  10 minutes ago  8   0.00%  0.00%  18.200.200.99  10.1.1.1
    ip-10-1-2-2.ec2.internal      running  10 minutes ago  6   0.00%  0.00%  3.80.240.200   10.1.2.2
    ip-10-1-3-3.ec2.internal      running  10 minutes ago  8   0.00%  0.00%  3.90.90.200    10.1.3.3
```

## Attributes

| Attribute  | Description                                    |
| ---------- | ---------------------------------------------- |
| **id**     | The unique identifier of the instance          |
| **status** | The current status of the instance (running)   |

## Management

Instance types and counts are managed through [Rack Parameters](/configuration/rack-parameters). Use `node_type` to change the instance size and scaling parameters to control the number of instances in your cluster.

### Connecting to an Instance
```bash
$ convox instances ssh i-0a1b2c3d4e5f
```

### Terminating an Instance
```bash
$ convox instances terminate i-0a1b2c3d4e5f
```
Terminated instances are automatically replaced by the cluster autoscaler.