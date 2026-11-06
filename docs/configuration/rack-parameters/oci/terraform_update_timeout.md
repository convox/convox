---
title: "terraform_update_timeout"
description: "The terraform_update_timeout Oracle Cloud rack parameter sets how long Terraform waits for OKE node pool updates to finish, defaulting to 2h."
slug: terraform_update_timeout
url: /configuration/rack-parameters/oci/terraform_update_timeout
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
Updating parameters... OK
```
This command sets the Terraform update timeout to 3 hours.

## Additional Information
The value must be a valid Go duration string (e.g., `2h`, `90m`, `2h30m`). This timeout applies to the OKE node pool update operation. The default value of `2h` matches the previously hardcoded behavior, so existing racks are unaffected.

## See Also
- [node_type](/configuration/rack-parameters/oci/node_type) for configuring the node compute shape
- [syslog](/configuration/rack-parameters/oci/syslog) for forwarding logs to an external syslog endpoint
