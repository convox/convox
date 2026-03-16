---
title: "proxy_protocol"
slug: proxy_protocol
url: /configuration/rack-parameters/azure/proxy_protocol
---

# proxy_protocol

## Description
The `proxy_protocol` parameter enables or disables the PROXY protocol on the nginx ingress controller. When enabled, nginx will use the PROXY protocol to preserve the original client IP address when traffic passes through the load balancer.

## Default Value
The default value for `proxy_protocol` is `false`.

## Use Cases
- **Client IP Preservation**: Maintain the real client IP address for logging and access control.
- **Security Policies**: Implement IP-based security rules that depend on knowing the true client IP.
- **Analytics**: Ensure accurate geographic or IP-based analytics data.

## Setting Parameters
To set the `proxy_protocol` parameter, use the following command:
```bash
$ convox rack params set proxy_protocol=true -r rackName
Setting parameters... OK
```

## Additional Information
Enabling proxy protocol sets `use-proxy-protocol: true` in the nginx configuration. Ensure your load balancer is configured to send PROXY protocol headers, otherwise connections may fail.
