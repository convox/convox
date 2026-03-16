---
title: "tags"
slug: tags
url: /configuration/rack-parameters/azure/tags
---

# tags

## Description
The `tags` parameter allows you to apply custom Azure resource tags to the rack's resource group and load balancer resources. Tags are key-value pairs useful for cost tracking, organization, and compliance.

## Default Value
The default value is an empty string (`""`), which means no additional tags are applied beyond the system defaults (`System=convox`, `Rack=<name>`).

## Use Cases
- **Cost Allocation**: Tag resources with cost center or project identifiers for billing reports.
- **Environment Classification**: Identify resources by environment (e.g., `env=production`).
- **Compliance**: Apply mandatory organizational tags required by governance policies.

## Setting Parameters
The value should be a comma-separated list of `key=value` pairs. It can be provided as plain text or base64-encoded:
```bash
$ convox rack params set tags=env=production,team=platform -r rackName
Setting parameters... OK
```

## Additional Information
Custom tags are merged with the default Convox tags (`System=convox`, `Rack=<rack-name>`). Tags are applied to the Azure resource group and propagated to load balancer annotations. Tag keys and values must comply with Azure tagging requirements.
