---
title: "nginx_additional_config"
draft: false
slug: nginx_additional_config
url: /configuration/rack-parameters/azure/nginx_additional_config
---

# nginx_additional_config

## Description
The `nginx_additional_config` parameter allows you to pass additional key-value configuration pairs to the nginx ingress controller's ConfigMap. This provides fine-grained control over nginx behavior beyond the standard parameters.

## Default Value
The default value is an empty string (`""`).

## Use Cases
- **Custom Timeouts**: Set custom proxy read/write timeouts.
- **Rate Limiting**: Configure request rate limiting at the ingress level.
- **Custom Headers**: Add or modify HTTP headers in the nginx configuration.
- **Buffering**: Adjust proxy buffering settings for specific workloads.

## Setting Parameters
The value should be a comma-separated list of `key=value` pairs. It can be provided as plain text or base64-encoded:
```html
$ convox rack params set nginx_additional_config=proxy-read-timeout=300,proxy-send-timeout=300 -r rackName
Setting parameters... OK
```

## Additional Information
The configuration pairs are merged into the nginx ConfigMap. Refer to the [nginx ingress controller documentation](https://kubernetes.github.io/ingress-nginx/user-guide/nginx-configuration/configmap/) for available configuration options.
