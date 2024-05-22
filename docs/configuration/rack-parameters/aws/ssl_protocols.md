---
title: "ssl_protocols"
draft: false
slug: ssl_protocols
url: /configuration/rack-parameters/aws/ssl_protocols
---

# ssl_protocols

## Description
The `ssl_protocols` parameter specifies the SSL protocols to use for [nginx](https://nginx.org/en/docs/http/ngx_http_proxy_module.html#proxy_ssl_protocols) (e.g. **TLSv1.2 TLSv1.3**). They must be separated by spaces.

## Default Value
The default value for `ssl_protocols` is ``. When set to ``, Nginx will use its default protocols: `TLSv1.2 TLSv1.3`.

## Use Cases
- **Security Compliance**: Specify custom SSL protocols to comply with organizational security policies or regulatory requirements.
- **Compatibility**: Choose protocols that ensure compatibility with clients and systems interacting with your application.

## Setting Parameters
To set the `ssl_protocols` parameter, use the following command:
```html
$ convox rack params set ssl_protocols='TLSv1.2 TLSv1.3' -r rackName
Setting parameters... OK
```
This command sets the SSL protocols to the specified values.

## Additional Information
Configuring the appropriate SSL protocols is essential for maintaining the security and compatibility of your application. By setting the `ssl_protocols` parameter, you can ensure that your application uses the desired protocols for secure communication. For more information on SSL protocols, refer to the [nginx documentation](https://nginx.org/en/docs/http/ngx_http_proxy_module.html#proxy_ssl_protocols).
