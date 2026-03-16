---
title: "build_node_type"
slug: build_node_type
url: /configuration/rack-parameters/aws/build_node_type
---

# build_node_type

## Description
The `build_node_type` parameter specifies the instance type for the build node. By default, it uses the same value as the `node_type` parameter unless explicitly set otherwise.

## Default Value
The default value for `build_node_type` is the same as the [node_type](/configuration/rack-parameters/aws/node_type) parameter.

## Use Cases
- **Custom Build Configuration**: Allows you to specify a different instance type for build nodes, optimizing them for build tasks which might have different requirements compared to runtime nodes.
- **Performance Optimization**: You can choose an instance type with higher compute or memory resources to speed up build processes.

## Setting Parameters
To set the `build_node_type` parameter, use the following command:
```bash
$ convox rack params set build_node_type=c5.large -r rackName
Setting parameters... OK
```
This command sets the build node type to `c5.large`.

## Architecture Compatibility

The `build_node_type` must use the same CPU architecture as the [node_type](/configuration/rack-parameters/aws/node_type) parameter. If your rack uses x86 instances (e.g. `t3`, `c5`, `m5`), the build node must also be x86. If your rack uses ARM/Graviton instances (e.g. `t4g`, `c6g`, `m6g`), the build node must also be ARM.

Mixing architectures (for example, `node_type=t3.small` with `build_node_type=t4g.large`) will cause build failures because the built container images will target the wrong CPU architecture for the nodes that run them.

When `build_node_type` is not set, it defaults to the value of `node_type`, which avoids this issue.

## Additional Information
Selecting the appropriate `build_node_type` can significantly impact the performance and cost of your build processes. Consider the resource requirements of your builds when choosing an instance type.
