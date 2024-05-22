---
title: "min_on_demand_count"
draft: false
slug: min_on_demand_count
url: /configuration/rack-parameters/aws/min_on_demand_count
---

# min_on_demand_count

## Description
The `min_on_demand_count` parameter sets the minimum number of on-demand nodes when using the `mixed` node capacity type. This allows you to ensure a baseline of on-demand instances in your cluster.

## Default Value
The default value for `min_on_demand_count` is `1`.

## Use Cases
- **Reliability**: Ensure that a minimum number of reliable on-demand instances are always available to handle workloads.
- **Performance Assurance**: Maintain a specific number of on-demand nodes to meet performance and reliability requirements.

## Setting Parameters
To set the `min_on_demand_count` parameter, use the following command:
```html
$ convox rack params set min_on_demand_count=2 -r rackName
Setting parameters... OK
```
This command sets the minimum number of on-demand nodes to 2.

## Additional Information
The `min_on_demand_count` parameter is used in conjunction with the [node_capacity_type](/configuration/rack-parameters/aws/node_capacity_type) parameter. When the `node_capacity_type` is set to `mixed`, you can specify the minimum and maximum number of on-demand nodes to balance cost and availability.

Adjusting the `min_on_demand_count` helps you ensure that there are always a sufficient number of reliable on-demand nodes available for your workloads, complementing the use of spot instances to reduce costs.

Additionally, consider configuring the [max_on_demand_count](/configuration/rack-parameters/aws/max_on_demand_count) parameter to limit the maximum number of on-demand nodes and optimize resource allocation.

By configuring the `min_on_demand_count` parameter, you can effectively manage the reliability and performance of your Convox rack.
