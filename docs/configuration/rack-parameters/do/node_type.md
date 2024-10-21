---
title: "node_type"
draft: false
slug: node_type
url: /configuration/rack-parameters/do/node_type
---

# node_type

## Description
The `node_type` parameter specifies the [node instance type](https://slugs.do-api.dev/) to use for nodes in your Convox rack. This allows you to choose the appropriate instance type based on your application's requirements.

## Default Value
The default value for `node_type` is `s-2vcpu-4gb`.

## Use Cases
- **Performance Optimization**: Select an instance type that provides the necessary CPU, memory, and network performance for your application.
- **Cost Management**: Choose an instance type that balances cost with the required performance characteristics.

## Setting Parameters
To set the `node_type` parameter, use the following command:
```html
$ convox rack params set node_type=s-2vcpu-4gb -r rackName
Setting parameters... OK
```
This command sets the `node_type` parameter to the specified value.

## Additional Information
Selecting the appropriate `node_type` is crucial for ensuring that your applications run efficiently and cost-effectively. Consider the specific needs of your workload when choosing an instance type. For more information on Digital Ocean instance types, refer to the [Digital Ocean documentation](https://slugs.do-api.dev/).
