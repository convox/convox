---
title: "nginx_additional_config"
description: "The nginx_additional_config AWS rack parameter passes extra key-value pairs to the nginx ingress controller ConfigMap for fine-grained control over nginx behavior."
slug: nginx_additional_config
url: /configuration/rack-parameters/aws/nginx_additional_config
---

# nginx_additional_config

## Description
The `nginx_additional_config` parameter passes additional key-value configuration pairs to the nginx ingress controller's ConfigMap. This provides fine-grained control over nginx behavior beyond the standard Rack parameters.

This parameter applies when [router_type](/configuration/rack-parameters/aws/router_type) is `nginx`, the default for AWS Racks. When `router_type=contour`, the parameter has no effect and the CLI prints a warning.

## Default Value
The default value is an empty string (`""`).

## Use Cases
- **Custom Timeouts**: Set custom proxy read/write timeouts.
- **Rate Limiting**: Configure request rate limiting at the ingress level.
- **Custom Headers**: Add or modify HTTP headers in the nginx configuration.
- **Buffering**: Adjust proxy buffering settings for specific workloads.

## Setting Parameters
The value is a comma-separated list of `key=value` pairs. It can be provided as plain text or base64-encoded. Base64 encoding avoids shell quoting issues when the configuration string contains special characters.

```bash
$ convox rack params set nginx_additional_config=proxy-read-timeout=300,proxy-send-timeout=300 -r rackName
Updating parameters... OK
```

Keys and values must not themselves contain commas or equals signs, because the Rack splits the string on those characters. Base64 encoding does not lift this restriction since the value is decoded before it is split.

To remove all custom entries and return to the default nginx configuration, set the parameter to an empty value:

```bash
$ convox rack params set nginx_additional_config= -r rackName
Updating parameters... OK
```

## Additional Information
This parameter is available since Rack version `3.23.1`.

The configuration pairs are merged into the `nginx-configuration` ConfigMap consumed by the nginx ingress controller. If a supplied key matches a setting Convox already manages (such as `proxy-body-size`), the supplied value takes precedence, so override managed keys with care.

The entries apply to the public router only. The internal router reads a separate ConfigMap (`nginx-internal-configuration`) that does not receive these entries.

Refer to the [nginx ingress controller documentation](https://kubernetes.github.io/ingress-nginx/user-guide/nginx-configuration/configmap/) for available configuration options.

## See Also
- [router_type](/configuration/rack-parameters/aws/router_type)
- [Ingress Router](/configuration/ingress-router)
- [ssl_ciphers](/configuration/rack-parameters/aws/ssl_ciphers)
- [ssl_protocols](/configuration/rack-parameters/aws/ssl_protocols)
