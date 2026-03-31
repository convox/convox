---
title: "terraform_update_timeout"
slug: terraform_update_timeout
url: /configuration/rack-parameters/do/terraform_update_timeout
---

# terraform_update_timeout

## Description
The `terraform_update_timeout` parameter controls how long Terraform waits for DOKS cluster update operations to complete. On large clusters, updates can take longer than the default timeout.

## Default Value
The default value for `terraform_update_timeout` is `2h` (2 hours).

## Use Cases
- **Large Clusters**: Extending the timeout for clusters where updates take longer than 2 hours.
- **Version Upgrades**: Kubernetes version upgrades on large clusters can exceed default timeouts.

## Setting Parameters
To set the `terraform_update_timeout` parameter, use the following command:
```bash
$ convox rack params set terraform_update_timeout=3h -r rackName
Setting parameters... OK
```
This command sets the Terraform update timeout to 3 hours.

## Additional Information
The value must be a valid Go duration string (e.g., `2h`, `90m`, `2h30m`). This timeout applies to the DigitalOcean Kubernetes cluster resource, which includes the node pool. The default value of `2h` matches the previously hardcoded behavior, so existing racks are unaffected.

## See Also
- [node_type](/configuration/rack-parameters/do/node_type) for configuring the node instance type
- [high_availability](/configuration/rack-parameters/do/high_availability) for enabling high availability mode
