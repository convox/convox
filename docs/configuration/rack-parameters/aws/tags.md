---
title: "tags"
draft: false
slug: tags
url: /configuration/rack-parameters/aws/tags
---

# tags

## Description
The `tags` parameter specifies custom tags to add to AWS resources (e.g. **key1=val1,key2=val2**).

## Default Value
The default value for `tags` is `null`. When set to `null`, no custom tags are applied. Convox managed tags are always applied, and custom tags specified with this parameter are appended to the Convox managed tags. This parameter is optional and can be configured based on your specific tagging needs.

## Use Cases
- **Resource Organization**: Use custom tags to organize and identify AWS resources for billing, management, and operational purposes.
- **Cost Allocation**: Apply tags for cost allocation and chargeback processes within your organization.

## Managing Parameters

### Viewing Current Parameters
```html
$ convox rack params -r rackName
tags  
```

### Setting Parameters
To set the `tags` parameter, use the following command:
```html
$ convox rack params set tags=key1=val1,key2=val2 -r rackName
Setting parameters... OK
```
This command sets the custom tags to the specified values.

## Additional Information
By configuring the `tags` parameter, you can add custom tags to AWS resources managed by Convox. These custom tags are appended to the existing Convox managed tags, providing additional metadata for organizing and managing your resources. Ensure that the tags are formatted correctly and meet the requirements of your organization's tagging policies. For more information on tagging AWS resources, refer to the [AWS documentation on tagging](https://docs.aws.amazon.com/general/latest/gr/aws_tagging.html).
