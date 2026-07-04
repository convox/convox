---
title: "nginx_additional_config"
description: "The nginx_additional_config GCP rack parameter passes extra key-value pairs to the nginx ingress controller ConfigMap for fine-grained control over nginx behavior."
slug: nginx_additional_config
url: /configuration/rack-parameters/gcp/nginx_additional_config
---

# nginx_additional_config

## Description
The `nginx_additional_config` parameter passes additional key-value configuration pairs to the nginx ingress controller's ConfigMap. GCP Racks route all traffic through the nginx ingress controller, and this parameter provides fine-grained control over nginx behavior beyond the standard parameters. Available since Rack version 3.23.1.

## Default Value
The default value is an empty string (`""`).

## Use Cases
- **Custom Timeouts**: Set custom proxy read/write timeouts for slow upstream Services.
- **Rate Limiting**: Configure request rate limiting at the ingress level.
- **Custom Headers**: Add or modify HTTP headers in the nginx configuration.
- **Buffering**: Adjust proxy buffering settings for specific workloads.

## Setting Parameters
The value is a comma-separated list of `key=value` pairs. It can be provided as plain text or base64-encoded:
```bash
$ convox rack params set nginx_additional_config=proxy-read-timeout=300,proxy-send-timeout=300 -r rackName
Updating parameters... OK
```

## Additional Information
The configuration pairs are merged into the `nginx-configuration` ConfigMap that the nginx ingress controller reads. Pairs you provide take precedence over the ConfigMap entries the Rack manages by default, such as `proxy-body-size`. Because the value is parsed by splitting on commas and equals signs, individual keys and values cannot contain commas or additional equals signs. Refer to the [nginx ingress controller documentation](https://kubernetes.github.io/ingress-nginx/user-guide/nginx-configuration/configmap/) for available configuration options.

## See Also
- [Rack Parameters](/configuration/rack-parameters)
- [nginx_additional_config (Azure)](/configuration/rack-parameters/azure/nginx_additional_config)
