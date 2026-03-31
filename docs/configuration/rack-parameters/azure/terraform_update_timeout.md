---
title: "terraform_update_timeout"
slug: terraform_update_timeout
url: /configuration/rack-parameters/azure/terraform_update_timeout
---

# terraform_update_timeout

## Description
The `terraform_update_timeout` parameter controls how long Terraform waits for AKS cluster and node pool update operations to complete. On large clusters, node pool updates can take longer than the default timeout.

## Default Value
The default value for `terraform_update_timeout` is `2h` (2 hours).

## Use Cases
- **Large Clusters**: Extending the timeout for clusters where node pool updates take longer than 2 hours.
- **Version Upgrades**: AKS version upgrades on large clusters can exceed default timeouts.

## Setting Parameters
To set the `terraform_update_timeout` parameter, use the following command:
```bash
$ convox rack params set terraform_update_timeout=3h -r rackName
Setting parameters... OK
```
This command sets the Terraform update timeout to 3 hours.

## Additional Information
The value must be a valid Go duration string (e.g., `2h`, `90m`, `2h30m`). This timeout applies to the AKS cluster resource and all additional node pool resources. The default value of `2h` matches the previously hardcoded behavior, so existing racks are unaffected.

## See Also
- [additional_node_groups_config](/configuration/rack-parameters/azure/additional_node_groups_config) for configuring additional node pools (all respect this timeout)
- [additional_build_groups_config](/configuration/rack-parameters/azure/additional_build_groups_config) for configuring build node pools
