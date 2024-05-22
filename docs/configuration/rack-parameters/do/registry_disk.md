---
title: "registry_disk"
draft: false
slug: registry_disk
url: /configuration/rack-parameters/do/registry_disk
---

# registry_disk

## Description
The `registry_disk` parameter specifies the size of the registry disk used in your Convox rack.

## Default Value
The default value for `registry_disk` is `50Gi`.

## Use Cases
- **Resource Configuration**: Ensure that your Convox rack is configured according to your needs and Digital Ocean best practices.

## Setting Parameters
To set the `registry_disk` parameter, use the following command:
```html
$ convox rack params set registry_disk=50Gi -r rackName
Setting parameters... OK
```
This command sets the `registry_disk` parameter to the specified value.

## Additional Information
Adjusting the size of the registry disk allows you to manage the storage capacity available for your images. Ensure that the specified size meets the storage requirements of your applications and services.
