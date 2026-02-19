---
title: "node_disk"
draft: false
slug: node_disk
url: /configuration/rack-parameters/azure/node_disk
---

# node_disk

## Description
Sets the OS disk size in GB for nodes in the default AKS node pool. Also used as the default disk size for additional node pools when no `disk` value is specified.

## Default Value
`30`

## Example
```html
$ convox rack params set node_disk=50 -r rackName
Setting parameters... OK
```
