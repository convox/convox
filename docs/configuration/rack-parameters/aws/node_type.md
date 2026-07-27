---
title: "node_type"
description: "The node_type AWS rack parameter sets the EC2 instance type for cluster nodes, fixing their compute, memory, and CPU architecture, defaulting to t3.small."
slug: node_type
url: /configuration/rack-parameters/aws/node_type
---

# node_type

## Description
The `node_type` parameter specifies the instance type for the nodes in the cluster. This determines the compute, memory, and network resources allocated to each node.

## Default Value
The default value for `node_type` is `t3.small`.

## Use Cases
- **Resource Allocation**: Choose an instance type that matches the resource requirements of your applications.
- **Performance Optimization**: Select instance types that provide the necessary compute power and memory to ensure optimal performance.

## Setting Parameters
To set the `node_type` parameter, use the following command:
```bash
$ convox rack params set node_type=c5.large -r rackName
Updating parameters... OK
```
This command sets the node instance type to `c5.large`.

## CPU Architecture (x86 vs ARM)

Convox supports both x86 (Intel/AMD) and ARM (Graviton) instance types. The `node_type` sets the CPU architecture for the Rack's system components, and it is the default architecture for [`build_node_type`](/configuration/rack-parameters/aws/build_node_type) and, on [Karpenter](/configuration/scaling/karpenter) Racks, for [`karpenter_arch`](/configuration/rack-parameters/aws/karpenter_arch) when that parameter is left unset.

**x86 instance families** (default): `t3`, `c5`, `m5`, `r5`, `c6i`, `m6i`, etc.

**ARM/Graviton instance families**: `t4g`, `c6g`, `c7g`, `m6g`, `r6g`, `a1`, etc.

Node groups added with [`additional_node_groups_config`](/configuration/rack-parameters/aws/additional_node_groups_config) and [`additional_build_groups_config`](/configuration/rack-parameters/aws/additional_build_groups_config) may use a different CPU architecture from `node_type`. Convox selects an arm64 or x86 EKS AMI for each node group from that node group's own instance type, so x86 and ARM node groups can coexist in one Rack.

What does not follow automatically is your App's image. An image built for one architecture will not run on nodes of the other, and Processes scheduled onto a mismatched node fail with an exec format error. On a mixed-architecture Rack, use the [`BuildArch`](/configuration/app-parameters/aws/BuildArch) app parameter to pin each App's image to the right architecture and `nodeSelectorLabels` to keep its Services on matching nodes. See [Workload Placement](/configuration/scaling/workload-placement).

## Additional Information
Selecting the appropriate instance type for your nodes is crucial for achieving the desired performance and cost-efficiency. AWS offers a variety of instance types, each with different combinations of CPU, memory, storage, and networking capacity. Consider your application's specific needs when choosing an instance type. For more information on AWS EC2 instance types, refer to the [AWS documentation on EC2 instance types](https://docs.aws.amazon.com/ec2/latest/instancetypes/ec2-instance-type-specifications.html).
