---
title: "node_capacity_type"
draft: false
slug: node_capacity_type
url: /configuration/rack-parameters/aws/node_capacity_type
---

# node_capacity_type

## Description
The `node_capacity_type` parameter specifies the type of capacity for the cluster nodes. It can be set to `on_demand`, `spot`, or `mixed`. 

- `on_demand`: Uses AWS on-demand instances for the cluster nodes.
- `spot`: Uses AWS spot instances for the cluster nodes.
- `mixed`: Creates one node group with on-demand instances and the other two with spot instances. Use `mixed` with the [min_on_demand_count](/configuration/rack-parameters/aws/min_on_demand_count) and [max_on_demand_count](/configuration/rack-parameters/aws/max_on_demand_count) parameters to control the minimum and maximum number of on-demand nodes, ensuring acceptable service availability even if spot instances become unavailable.

## Default Value
The default value for `node_capacity_type` is `on_demand`.

## Use Cases
- **Cost Optimization**: Use `spot` or `mixed` capacity types to reduce costs by leveraging lower-cost spot instances while maintaining a baseline of reliable on-demand instances.
- **Reliability and Performance**: Use `on_demand` capacity type for maximum reliability and performance, as on-demand instances are not subject to interruption like spot instances.

## Managing Parameters

### Viewing Current Parameters
```html
$ convox rack params -r rackName
node_capacity_type  on_demand
node_disk  20
node_type  t3.small
```

### Setting Parameters
To set the `node_capacity_type` parameter, use the following command:
```html
$ convox rack params set node_capacity_type=mixed -r rackName
Setting parameters... OK
```
This command sets the node capacity type to `mixed`.

## Additional Information
When using the `mixed` capacity type, it is important to configure the [min_on_demand_count](/configuration/rack-parameters/aws/min_on_demand_count) and [max_on_demand_count](/configuration/rack-parameters/aws/max_on_demand_count) parameters to ensure that your cluster maintains the desired balance of cost and reliability.

Using spot instances can lead to potential volatility as these instances may be interrupted by AWS when capacity is needed elsewhere. This setup is recommended only for non-production environments that can tolerate such interruptions.

By setting the `node_capacity_type` parameter appropriately, you can optimize the cost-efficiency and reliability of your Convox rack based on your specific needs and workload requirements.
