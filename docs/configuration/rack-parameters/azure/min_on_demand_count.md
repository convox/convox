---
title: "min_on_demand_count"
slug: min_on_demand_count
url: /configuration/rack-parameters/azure/min_on_demand_count
---

# min_on_demand_count

## Description
The `min_on_demand_count` parameter sets the minimum number of nodes in the default AKS node pool. This value is also used as the initial `node_count` when the cluster is first created.

## Default Value
The default value for `min_on_demand_count` is `3`.

## Use Cases
- **Baseline Availability**: Ensure a minimum number of nodes are always running to handle steady-state traffic without waiting for the autoscaler to provision new nodes.
- **Initial Cluster Sizing**: Control the starting size of your node pool, since this value also determines the initial `node_count`.

## Setting Parameters
To set the `min_on_demand_count` parameter, use the following command:
```bash
$ convox rack params set min_on_demand_count=2 -r rackName
Setting parameters... OK
```
This command sets the minimum number of nodes in the default node pool to `2`.
