---
title: "idle_timeout"
draft: false
slug: idle_timeout
url: /configuration/rack-parameters/azure/idle_timeout
---

# idle_timeout

## Description
The `idle_timeout` parameter specifies the idle timeout (in minutes) for the Azure Load Balancer associated with the router. This controls how long idle connections are kept alive before being closed.

## Default Value
The default value for `idle_timeout` is `4` (minutes).

## Use Cases
- **Long-Lived Connections**: Increase the timeout for applications that use WebSockets or long-polling.
- **Resource Optimization**: Reduce the timeout to free up load balancer resources more quickly.
- **API Gateways**: Adjust based on expected client connection patterns.

## Setting Parameters
To set the `idle_timeout` parameter, use the following command:
```html
$ convox rack params set idle_timeout=10 -r rackName
Setting parameters... OK
```

## Additional Information
The Azure Load Balancer supports idle timeout values between 4 and 30 minutes. This setting applies to both the external and internal (if enabled) load balancers.
