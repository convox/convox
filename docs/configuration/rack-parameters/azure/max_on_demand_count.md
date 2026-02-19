---
title: "max_on_demand_count"
draft: false
slug: max_on_demand_count
url: /configuration/rack-parameters/azure/max_on_demand_count
---

# max_on_demand_count

## Description
Sets the maximum number of nodes in the default AKS node pool for autoscaling.

## Default Value
`100`

## Example
```html
$ convox rack params set max_on_demand_count=50 -r rackName
Setting parameters... OK
```
