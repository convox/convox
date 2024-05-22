---
title: "imds_http_tokens"
draft: false
slug: imds_http_tokens
url: /configuration/rack-parameters/aws/imds_http_tokens
---

# imds_http_tokens

## Description
The `imds_http_tokens` parameter determines whether the Instance Metadata Service requires session tokens (IMDSv2). This setting enhances the security of instance metadata by requiring a session token for access.

## Default Value
The default value for `imds_http_tokens` is `optional`.

## Use Cases
- **Enhanced Security**: Requiring IMDSv2 tokens helps protect against unauthorized metadata access.
- **Compliance**: Some security frameworks recommend or require the use of IMDSv2 for accessing instance metadata.

## Setting Parameters
To set the `imds_http_tokens` parameter, use the following command:
```html
$ convox rack params set imds_http_tokens=required -r rackName
Setting parameters... OK
```
This command sets the IMDSv2 tokens requirement to `required`.

## Additional Information
Instance Metadata Service Version 2 (IMDSv2) improves the security of metadata access by requiring a session token. This mitigates certain types of attacks, such as SSRF (Server-Side Request Forgery). Ensure that your applications and scripts that access instance metadata are updated to use IMDSv2 when this setting is enabled.

The `imds_http_tokens` parameter can be set to:
- `optional`: Allows access to the metadata service with or without a session token.
- `required`: Requires a session token to access the metadata service.

Setting `imds_http_tokens` to `required` ensures that all requests to the Instance Metadata Service are authenticated using a session token, providing an additional layer of security.
