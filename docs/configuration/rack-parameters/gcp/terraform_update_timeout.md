---
title: "terraform_update_timeout"
slug: terraform_update_timeout
url: /configuration/rack-parameters/gcp/terraform_update_timeout
---

# terraform_update_timeout

## Description
The `terraform_update_timeout` parameter controls how long Terraform waits for node pool update operations to complete. On large clusters, node pool updates can take longer than the default timeout.

## Default Value
The default value for `terraform_update_timeout` is `2h` (2 hours).

## Use Cases
- **Large Clusters**: Extending the timeout for clusters where node pool updates take longer than 2 hours.
- **Slow Rolling Updates**: When upgrade settings throttle node replacements, increasing the total update time.

## Setting Parameters
To set the `terraform_update_timeout` parameter, use the following command:
```bash
$ convox rack params set terraform_update_timeout=3h -r rackName
Setting parameters... OK
```
This command sets the Terraform update timeout to 3 hours.

## Additional Information
The value must be a valid Go duration string (e.g., `2h`, `90m`, `2h30m`). This timeout applies to the GKE node pool update operation.
