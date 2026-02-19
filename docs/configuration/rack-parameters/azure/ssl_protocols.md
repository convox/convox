---
title: "ssl_protocols"
draft: false
slug: ssl_protocols
url: /configuration/rack-parameters/azure/ssl_protocols
---

# ssl_protocols

## Description
The `ssl_protocols` parameter specifies which SSL/TLS protocol versions the nginx ingress controller should accept. This allows you to control the minimum and maximum TLS versions for HTTPS connections.

## Default Value
The default value is an empty string (`""`), which uses the nginx default protocols.

## Use Cases
- **Security Compliance**: Enforce TLS 1.2+ to meet compliance standards.
- **Legacy Support**: Allow older TLS versions if needed for backward compatibility.
- **Hardening**: Disable older protocols like TLS 1.0 and 1.1 to reduce attack surface.

## Setting Parameters
To set the `ssl_protocols` parameter, use the following command:
```html
$ convox rack params set ssl_protocols=TLSv1.2+TLSv1.3 -r rackName
Setting parameters... OK
```

## Additional Information
The value is passed directly to the nginx `ssl-protocols` configuration directive. Common values include `TLSv1.2`, `TLSv1.3`, or `TLSv1.2 TLSv1.3`. Use `+` to separate multiple protocols (they are converted to spaces internally).
