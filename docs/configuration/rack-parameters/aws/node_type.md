---
title: "node_type"
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
Setting parameters... OK
```
This command sets the node instance type to `c5.large`.

## CPU Architecture (x86 vs ARM)

Convox supports both x86 (Intel/AMD) and ARM (Graviton) instance types. The `node_type` determines the CPU architecture for the entire rack, including system components, release images, and logging agents.

**x86 instance families** (default): `t3`, `c5`, `m5`, `r5`, `c6i`, `m6i`, etc.

**ARM/Graviton instance families**: `t4g`, `c6g`, `c7g`, `m6g`, `r6g`, `a1`, etc.

> All node groups in a rack must use the same CPU architecture. Do not mix x86 and ARM instance types between `node_type`, `build_node_type`, `additional_node_groups_config`, or `additional_build_groups_config`. Convox selects AMIs, system images, and build tooling based on the architecture of `node_type`. A mismatch will cause pod scheduling failures and build errors.

For example, if `node_type` is set to a Graviton instance like `t4g.medium`, then `build_node_type` and any additional node or build groups must also use ARM-based instance types.

## Additional Information
Selecting the appropriate instance type for your nodes is crucial for achieving the desired performance and cost-efficiency. AWS offers a variety of instance types, each with different combinations of CPU, memory, storage, and networking capacity. Consider your application's specific needs when choosing an instance type. For more information on AWS EC2 instance types, refer to the [AWS documentation on EC2 instance types](https://docs.aws.amazon.com/ec2/latest/instancetypes/ec2-instance-type-specifications.html).
