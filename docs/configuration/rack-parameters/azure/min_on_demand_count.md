---
title: "min_on_demand_count"
draft: false
slug: min_on_demand_count
url: /configuration/rack-parameters/azure/min_on_demand_count
---

# min_on_demand_count

## Description
Sets the minimum number of nodes in the default AKS node pool. This is also used as the initial `node_count`.

## Default Value
`3`

## Example
```html
$ convox rack params set min_on_demand_count=2 -r rackName
Setting parameters... OK
```
