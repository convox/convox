---
title: "build_node_type"
draft: false
slug: build_node_type
url: /configuration/rack-parameters/aws/build_node_type
---

# build_node_type

## Description
The `build_node_type` parameter specifies the instance type for the build node. By default, it uses the same value as the `node_type` parameter unless explicitly set otherwise.

## Default Value
The default value for `build_node_type` is the same as the `node_type` parameter.

## Use Cases
- **Custom Build Configuration**: Allows you to specify a different instance type for build nodes, optimizing them for build tasks which might have different requirements compared to runtime nodes.
- **Performance Optimization**: You can choose an instance type with higher compute or memory resources to speed up build processes.

## Managing Parameters

### Viewing Current Parameters
```html
$ convox rack params -r rackName
build_node_type  t3.small
node_disk  20
node_type  t3.small
```

### Setting Parameters
To set the `build_node_type` parameter, use the following command:
```html
$ convox rack params set build_node_type=c5.large -r rackName
Setting parameters... OK
```
This command sets the build node type to `c5.large`.

## Additional Information
Selecting the appropriate `build_node_type` can significantly impact the performance and cost of your build processes. Consider the resource requirements of your builds when choosing an instance type. If not set, the build node will default to the type specified by the `node_type` parameter, ensuring consistency across your infrastructure.
