---
title: "node_max_unavailable_percentage"
slug: node_max_unavailable_percentage
url: /configuration/rack-parameters/aws/node_max_unavailable_percentage
---

# node_max_unavailable_percentage

## Description
The `node_max_unavailable_percentage` parameter controls the maximum percentage of nodes that can be unavailable simultaneously during EKS node group updates. This applies to operations like Kubernetes version upgrades, AMI updates, and node group scaling changes.

When set, this value is applied to the `update_config` block on all EKS managed node groups (primary, build, and additional).

## Default Value
The default value for `node_max_unavailable_percentage` is `0` (disabled).

When set to `0`, no explicit update configuration is applied and AWS EKS uses its default node group update behavior.

## Use Cases
- **Faster Updates**: Set a higher percentage (e.g., 50) to allow more nodes to update simultaneously, reducing the total time for node group updates.
- **Stability**: Set a lower percentage (e.g., 10-25) to limit the blast radius of node updates, keeping more of the cluster available during rolling updates.
- **Cost vs Speed**: Higher values speed up updates but temporarily reduce available capacity. Lower values preserve capacity but slow down the update process.

## Setting Parameters
To set the node max unavailable percentage:
```bash
$ convox rack params set node_max_unavailable_percentage=25 -r rackName
Setting parameters... OK
```

The value must be between 1 and 100.

To disable the explicit update configuration and revert to AWS defaults:
```bash
$ convox rack params set node_max_unavailable_percentage=0 -r rackName
Setting parameters... OK
```

## Additional Information
- This parameter affects **EKS node group updates** (infrastructure-level node rolling), not application pod disruptions. For controlling pod-level disruptions, see [pdb_default_min_available_percentage](/configuration/rack-parameters/aws/pdb_default_min_available_percentage).
- The setting applies uniformly to all node groups in the rack: primary nodes, build nodes, and any additional node groups configured via [additional_node_groups_config](/configuration/rack-parameters/aws/additional_node_groups_config).
- When combined with [high_availability](/configuration/rack-parameters/aws/high_availability), ensure the percentage allows enough nodes to remain available to run your workloads during updates.

## See Also
- [terraform_update_timeout](/configuration/rack-parameters/aws/terraform_update_timeout) controls how long Terraform waits for node group updates to complete (increase if high unavailable percentage still causes timeouts)
- [pdb_default_min_available_percentage](/configuration/rack-parameters/aws/pdb_default_min_available_percentage) for pod-level disruption budgets
- [high_availability](/configuration/rack-parameters/aws/high_availability) for cluster redundancy settings
- [key_pair_name](/configuration/rack-parameters/aws/key_pair_name) for SSH access to nodes
