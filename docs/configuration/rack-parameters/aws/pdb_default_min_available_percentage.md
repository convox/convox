---
title: "pdb_default_min_available_percentage"
draft: false
slug: pdb_default_min_available_percentage
url: /configuration/rack-parameters/aws/pdb_default_min_available_percentage
---

# pdb_default_min_available_percentage

## Description
The `pdb_default_min_available_percentage` parameter defines the default minimum percentage of available replicas that must be maintained for a  Pod Disruption Budget(PDB) across all namespaces within a Rack. This setting allows for granular control over the level of disruption tolerated during pod disruptions, such as during node maintenance or upgrades.

## Default Value
The default value for `pdb_default_min_available_percentage` is `50`.

## Use Cases
- **Workload Protection**: Safeguard critical applications by setting a high minimum availability percentage to prevent disruptions that could impact service availability.
- **Resource Optimization**: Balance application availability with resource utilization by setting a lower minimum availability percentage for less critical workloads.
- **Compliance Adherence**: Ensure compliance with specific service level agreements (SLAs) or regulatory requirements by configuring the PDB to meet the desired availability level.

## Setting Parameters
To set the `pdb_default_min_available_percentage` parameter, use the following command:
```html
$ convox rack params set pdb_default_min_available_percentage=80 -r rackName
Setting parameters... OK
```

This command sets the default minimum availability to 80%(as an example value) for all PDBs within the specified Rack.

## Additional Information
The pdb_default_min_available_percentage parameter sets a minimum availability requirement for Pod Disruption Budgets (PDBs) across your Rack. This ensures a baseline level of protection for your applications.

By defining a default percentage, you can maintain consistent disruption tolerance across different workloads while allowing for customization when necessary. This parameter helps protect your applications from unexpected pod failures and disruptions, enhancing overall system stability.

