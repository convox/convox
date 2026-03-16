---
title: "pdb_default_min_available_percentage"
slug: pdb_default_min_available_percentage
url: /configuration/rack-parameters/azure/pdb_default_min_available_percentage
---

# pdb_default_min_available_percentage

## Description
The `pdb_default_min_available_percentage` parameter sets the default minimum available percentage for Pod Disruption Budgets (PDBs) created by Convox. This controls how many pods must remain available during voluntary disruptions such as node drains and cluster upgrades.

## Default Value
The default value for `pdb_default_min_available_percentage` is `50`.

## Use Cases
- **Cluster Upgrades**: Ensure a minimum percentage of pods remain running during rolling node upgrades.
- **Availability Tuning**: Increase the percentage for critical services that need higher availability during maintenance.
- **Flexibility**: Decrease the percentage to allow faster node drains when availability is less critical.

## Setting Parameters
To set the `pdb_default_min_available_percentage` parameter, use the following command:
```bash
$ convox rack params set pdb_default_min_available_percentage=75 -r rackName
Setting parameters... OK
```

## Additional Information
The value is a percentage (0-100) that determines the `minAvailable` field in Pod Disruption Budgets. A value of `50` means at least 50% of pods must be available during voluntary disruptions. This is passed as the `PDB_DEFAULT_MIN_AVAILABLE_PERCENTAGE` environment variable to the Convox API.
