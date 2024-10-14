---
title: "max_on_demand_count"
draft: false
slug: max_on_demand_count
url: /configuration/rack-parameters/aws/max_on_demand_count
---

# max_on_demand_count

## Description
The `max_on_demand_count` parameter sets the maximum number of on-demand nodes when using the `mixed` node capacity type. This allows you to control the upper limit of on-demand instances in your cluster.

## Default Value
The default value for `max_on_demand_count` is `100`.

## Use Cases
- **Cost Management**: Limit the number of on-demand instances to control costs while still benefiting from the reliability of on-demand nodes.
- **Resource Allocation**: Ensure that a specific maximum number of on-demand nodes are available for your cluster to meet performance and reliability requirements.

## Setting Parameters
To set the `max_on_demand_count` parameter, use the following command:
```html
$ convox rack params set max_on_demand_count=50 -r rackName
Setting parameters... OK
```
This command sets the maximum number of on-demand nodes to 50.

## Additional Information
The `max_on_demand_count` parameter is used in conjunction with the [node_capacity_type](/configuration/rack-parameters/aws/node_capacity_type) parameter. When the `node_capacity_type` is set to `mixed`, you can specify the minimum and maximum number of on-demand nodes to balance cost and availability.

Adjusting the `max_on_demand_count` helps you optimize your resource allocation by ensuring that you have a sufficient number of on-demand nodes for reliability while utilizing spot instances to reduce costs. 

Additionally, consider configuring the [min_on_demand_count](/configuration/rack-parameters/aws/min_on_demand_count) parameter to ensure that a minimum number of on-demand nodes are always available to handle your workloads.

By configuring the `max_on_demand_count` parameter, you can effectively manage the scalability and cost-efficiency of your Convox rack.