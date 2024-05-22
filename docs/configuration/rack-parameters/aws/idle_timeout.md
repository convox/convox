---
title: "idle_timeout"
draft: false
slug: idle_timeout
url: /configuration/rack-parameters/aws/idle_timeout
---

# idle_timeout

## Description
The `idle_timeout` parameter specifies the idle timeout value (in seconds) for the Rack Load Balancer. This setting determines how long the load balancer will wait before closing an idle connection.

## Default Value
The default value for `idle_timeout` is `3600` seconds (1 hour).

## Use Cases
- **Resource Optimization**: Adjusting the idle timeout can help optimize resource usage and performance for your applications.
- **Application Requirements**: Set an appropriate idle timeout based on your application's connection behavior and requirements.

## Setting Parameters
To set the `idle_timeout` parameter, use the following command:
```html
$ convox rack params set idle_timeout=600 -r rackName
Setting parameters... OK
```
This command sets the idle timeout value to 600 seconds (10 minutes).

## Additional Information
The idle timeout setting is crucial for managing the behavior of your load balancer and can impact the performance and efficiency of your applications. Shorter timeouts can help free up resources more quickly, but may also result in more frequent connection establishments. Longer timeouts can keep connections open longer, which might be necessary for applications with longer processing times.

When configuring the `idle_timeout` value, consider your application's specific needs and connection patterns to find the optimal balance between resource utilization and performance.