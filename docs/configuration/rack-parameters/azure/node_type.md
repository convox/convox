---
title: "node_type"
draft: false
slug: node_type
url: /configuration/rack-parameters/azure/node_type
---

# node_type

## Description
The `node_type` parameter specifies the type of instance to use for nodes in your Convox rack. This allows you to choose the appropriate instance type based on your application's requirements.

## Default Value
The default value for `node_type` is `Standard_D3_v3`.

## Use Cases
- **Performance Optimization**: Select an instance type that provides the necessary CPU, memory, and network performance for your application.
- **Cost Management**: Choose an instance type that balances cost with the required performance characteristics.

## Setting Parameters
To set the `node_type` parameter, use the following command:
```html
$ convox rack params set node_type=Standard_D3_v3 -r rackName
Setting parameters... OK
```
This command sets the `node_type` parameter to the specified value.

## Additional Information
Selecting the appropriate `node_type` is crucial for ensuring that your applications run efficiently and cost-effectively. Consider the specific needs of your workload when choosing an instance type. For more information on Azure instance types, refer to the [Azure documentation](https://azure.microsoft.com/en-us/pricing/details/virtual-machines/series/).
