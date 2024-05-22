---
title: "ssl_ciphers"
draft: false
slug: ssl_ciphers
url: /configuration/rack-parameters/aws/ssl_ciphers
---

# ssl_ciphers

## Description
The `ssl_ciphers` parameter specifies the SSL ciphers to use for [nginx](https://nginx.org/en/docs/http/ngx_http_ssl_module.html#ssl_ciphers). They must be separated by colons.

## Default Value
The default value for `ssl_ciphers` is ``. When set to ``, Nginx will use its default ciphers: `ECDHE-ECDSA-AES128-GCM-SHA256:ECDHE-RSA-AES128-GCM-SHA256:ECDHE-ECDSA-AES256-GCM-SHA384:ECDHE-RSA-AES256-GCM-SHA384:ECDHE-ECDSA-CHACHA20-POLY1305:ECDHE-RSA-CHACHA20-POLY1305:DHE-RSA-AES128-GCM-SHA256:DHE-RSA-AES256-GCM-SHA384`.

## Use Cases
- **Security Compliance**: Specify custom SSL ciphers to comply with organizational security policies or regulatory requirements.
- **Performance Optimization**: Choose ciphers that provide the best performance for your specific use case.

## Setting Parameters
To set the `ssl_ciphers` parameter, use the following command:
```html
$ convox rack params set ssl_ciphers=ECDHE-RSA-AES128-GCM-SHA256:ECDHE-RSA-AES256-GCM-SHA384 -r rackName
Setting parameters... OK
```
This command sets the SSL ciphers to the specified values.

## Additional Information
Selecting appropriate SSL ciphers is crucial for ensuring the security and performance of your application. By configuring the `ssl_ciphers` parameter, you can tailor the SSL settings to meet your specific requirements. For more information on SSL ciphers, refer to the [nginx documentation](https://nginx.org/en/docs/http/ngx_http_ssl_module.html#ssl_ciphers).
