---
title: "proxy_protocol"
draft: false
slug: proxy_protocol
url: /configuration/rack-parameters/aws/proxy_protocol
---

# proxy_protocol

## Description
The `proxy_protocol` parameter enables the Proxy Protocol. When this parameter is set, the client source IP will be available in the request header `x-forwarded-for` key.
**Requires 5 - 10 minutes downtime** 
This parameter is not applicable for the [internal_router](/configuration/rack-parameters/aws/internal_router) parameter.

## Default Value
The default value for `proxy_protocol` is `false`.

## Use Cases
- **Client IP Tracking**: Enable the Proxy Protocol to track the original client IP address in applications behind a load balancer.
- **Logging and Analytics**: Improve logging and analytics accuracy by capturing the client's source IP address.

## Setting Parameters
To set the `proxy_protocol` parameter, use the following command:
```html
$ convox rack params set proxy_protocol=true -r rackName
Setting parameters... OK
```
This command enables the Proxy Protocol for your rack.

## Additional Information
Enabling the Proxy Protocol requires a short downtime of 5 - 10 minutes as the load balancer configuration is updated. This parameter is useful for scenarios where you need to capture the original client IP address, which is often masked by the load balancer.

Note that the Proxy Protocol is not applicable when using the [internal_router](/configuration/rack-parameters/aws/internal_router) parameter.

By setting the `proxy_protocol` parameter, you can capture the original client IP address, enhancing your application's logging, analytics, and security capabilities.
