---
title: "node_type"
draft: false
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

## Managing Parameters

### Viewing Current Parameters
```html
$ convox rack params -r rackName
node_type  t3.small
node_disk  20
```

### Setting Parameters
To set the `node_type` parameter, use the following command:
```html
$ convox rack params set node_type=c5.large -r rackName
Setting parameters... OK
```
This command sets the node instance type to `c5.large`.

## Additional Information
Selecting the appropriate instance type for your nodes is crucial for achieving the desired performance and cost-efficiency. AWS offers a variety of instance types, each with different combinations of CPU, memory, storage, and networking capacity. Consider your application's specific needs when choosing an instance type. For more information on AWS EC2 instance types, refer to the [AWS documentation on EC2 instance types](https://docs.aws.amazon.com/ec2/latest/instancetypes/ec2-instance-type-specifications.html).

By setting the `node_type` parameter, you can optimize the compute and memory resources available to your Convox rack, ensuring it meets your workload requirements.
