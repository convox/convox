---
title: "ssl_ciphers"
draft: false
slug: ssl_ciphers
url: /configuration/rack-parameters/azure/ssl_ciphers
---

# ssl_ciphers

## Description
The `ssl_ciphers` parameter specifies the SSL/TLS cipher suites that the nginx ingress controller should use. This allows you to control which ciphers are available for HTTPS connections.

## Default Value
The default value is an empty string (`""`), which uses the nginx default cipher suite.

## Use Cases
- **Security Compliance**: Restrict ciphers to meet PCI DSS, HIPAA, or other compliance requirements.
- **Vulnerability Mitigation**: Disable weak ciphers to protect against known attacks.
- **Performance**: Prioritize faster cipher suites for improved TLS handshake performance.

## Setting Parameters
To set the `ssl_ciphers` parameter, use the following command:
```html
$ convox rack params set ssl_ciphers=ECDHE-RSA-AES128-GCM-SHA256:ECDHE-RSA-AES256-GCM-SHA384 -r rackName
Setting parameters... OK
```

## Additional Information
The value is passed directly to the nginx `ssl-ciphers` configuration directive. Refer to the [nginx documentation](https://nginx.org/en/docs/http/ngx_http_ssl_module.html#ssl_ciphers) for supported cipher strings.
