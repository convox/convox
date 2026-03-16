---
title: "max_on_demand_count"
slug: max_on_demand_count
url: /configuration/rack-parameters/azure/max_on_demand_count
---

# max_on_demand_count

## Description
The `max_on_demand_count` parameter sets the maximum number of nodes in the default AKS node pool for autoscaling. This defines the upper limit of nodes that the autoscaler can provision in response to increased workload demand.

## Default Value
The default value for `max_on_demand_count` is `100`.

## Use Cases
- **Cost Control**: Set an upper bound on the number of nodes to prevent unexpected cost increases during traffic spikes.
- **Capacity Planning**: Define the maximum cluster size to align with your organization's resource quotas and budget constraints.

## Setting Parameters
To set the `max_on_demand_count` parameter, use the following command:
```bash
$ convox rack params set max_on_demand_count=50 -r rackName
Setting parameters... OK
```
This command sets the maximum number of autoscaled nodes to `50`.
