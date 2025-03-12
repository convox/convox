---
title: "pdb_default_min_available_percentage"
draft: false
slug: pdb_default_min_available_percentage
url: /configuration/rack-parameters/aws/pdb_default_min_available_percentage
---

# pdb_default_min_available_percentage

## Description
The `pdb_default_min_available_percentage` parameter sets the default minimum percentage of available replicas that must be maintained for Pod Disruption Budgets (PDBs) across all namespaces within a Rack. This parameter helps manage pod disruptions during voluntary events such as node drains, updates, or autoscaling.

By configuring this parameter, you can control the balance between maintaining service availability and allowing efficient cluster operations like scaling down nodes when they're underutilized.

## Default Value
The default value for `pdb_default_min_available_percentage` is `50`.

## Use Cases
- **Optimized Node Scaling**: Configure lower percentages in environments with large scaling variance to allow unused nodes to be scaled down more quickly.
- **Cost Optimization**: Enable more aggressive node consolidation during low-traffic periods while maintaining an acceptable level of service availability.
- **High Availability**: Set higher percentages for production environments where service disruption must be minimized.
- **Cluster Updates**: Balance between maintaining service availability and enabling efficient cluster maintenance operations.

## Setting Parameters
To set the default minimum available percentage for Pod Disruption Budgets, use the following command:
```html
$ convox rack params set pdb_default_min_available_percentage=40 -r rackName
Setting parameters... OK
```

## Additional Information
- Pod Disruption Budgets (PDBs) protect applications from voluntary disruptions that might reduce availability below specified thresholds.
- This parameter sets a global default for all services in all namespaces within the Rack.
- Lower values (e.g., 25%) allow more aggressive scaling and faster node removal, which can reduce costs but potentially impact availability.
- Higher values (e.g., 75%) provide greater assurance of service availability during scaling events but may limit the ability to scale down quickly.
- The value represents the percentage of pods that must remain available during voluntary disruptions, relative to the desired number of replicas.
- For example, with a value of 50% and a service with 10 replicas, at least 5 replicas must remain available during any voluntary disruption.
- This parameter is particularly beneficial in environments with:
  - Large differences between peak and off-peak traffic
  - Cost-sensitive operations
  - Frequent scaling events
  - Automated node management
- The parameter affects only voluntary disruptions (like draining a node for updates) and does not provide protection against involuntary disruptions (like node failures).

## Version Requirements
This feature requires at least Convox rack version `3.18.8`.
