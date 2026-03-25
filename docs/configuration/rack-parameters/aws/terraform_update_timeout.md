---
title: "terraform_update_timeout"
slug: terraform_update_timeout
url: /configuration/rack-parameters/aws/terraform_update_timeout
---

# terraform_update_timeout

## Description
The `terraform_update_timeout` parameter controls how long Terraform waits for managed node group update operations to complete. On large clusters (50+ nodes), rack updates trigger rolling node replacements that can exceed the default 2-hour timeout. This parameter lets you extend that window.

## Default Value
The default value for `terraform_update_timeout` is `2h` (2 hours).

## Use Cases
- **Large Clusters**: Extending the timeout for clusters with 50+ nodes where rolling updates take longer than 2 hours.
- **Slow Rolling Updates**: When node replacements are throttled by `node_max_unavailable_percentage`, the total update time increases.

## Setting Parameters
To set the `terraform_update_timeout` parameter, use the following command:
```bash
$ convox rack params set terraform_update_timeout=3h -r rackName
Setting parameters... OK
```
This command sets the Terraform update timeout to 3 hours.

## Additional Information
The value must be a valid Go duration string (e.g., `2h`, `90m`, `2h30m`). This timeout applies to all EKS node group resources, including primary, build, and additional node groups. The default value of `2h` matches the previously hardcoded behavior, so existing racks are unaffected.
