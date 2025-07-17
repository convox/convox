---
title: "disable_convox_resolver"
draft: false
slug: disable_convox_resolver
url: /configuration/rack-parameters/aws/disable_convox_resolver
---

# disable_convox_resolver

## Description
The `disable_convox_resolver` parameter allows you to disable the Convox resolver and use the Kubernetes resolver instead for DNS resolution within your rack. This parameter is particularly useful when using `proxy_protocol=true` with internal service routing, as recent nginx version updates require the Kubernetes resolver to function properly in this configuration.

## Default Value
The default value for `disable_convox_resolver` is `false`, meaning the Convox resolver is enabled by default.

## Use Cases
- **Proxy Protocol Compatibility**: Required when using both `proxy_protocol=true` and internal service routing to ensure proper functionality.
- **Internal Service Communication**: Resolves compatibility issues with updated nginx versions when routing between internal services.
- **Kubernetes-Native DNS**: Allows applications to use the standard Kubernetes DNS resolver for service discovery.
- **Troubleshooting DNS Issues**: Provides an alternative DNS resolution method when experiencing issues with the Convox resolver.
- **Custom DNS Requirements**: Enables use of Kubernetes DNS features that may not be available through the Convox resolver.

## Setting Parameters
To disable the Convox resolver and use the Kubernetes resolver instead:
```html
$ convox rack params set disable_convox_resolver=true -r rackName
Setting parameters... OK
```

To re-enable the Convox resolver (default behavior):
```html
$ convox rack params set disable_convox_resolver=false -r rackName
Setting parameters... OK
```

## Common Configuration Pattern
This parameter is specifically required when you need to use both `proxy_protocol=true` and internal service routing:

```html
$ convox rack params set proxy_protocol=true -r rackName
$ convox rack params set disable_convox_resolver=true -r rackName
```

## Additional Information
- This parameter provides a solution for compatibility issues between the Convox resolver and `proxy_protocol=true` functionality.
- When disabled, the Convox resolver is replaced with the standard Kubernetes DNS resolver.
- The change affects all services within the rack and how they resolve DNS queries.
- Internal service routing requires the Kubernetes resolver when using proxy protocol with recent nginx versions.
- This setting maintains backward compatibility with existing configurations that do not use proxy protocol.
- The parameter can be toggled at any time without requiring application redeployment, though services may need to restart to pick up the new DNS configuration.

## DNS Resolution Behavior
- **Convox Resolver (default)**: Uses Convox's custom DNS resolution system for service discovery and external DNS queries.
- **Kubernetes Resolver (when disabled)**: Uses the standard Kubernetes cluster DNS system, typically CoreDNS, for all DNS resolution.

## Troubleshooting
If you experience DNS resolution issues or internal service communication problems, particularly when using proxy protocol, try enabling this parameter:

```html
$ convox rack params set disable_convox_resolver=true -r rackName
```

## Related Parameters
- [proxy_protocol](/configuration/rack-parameters/aws/proxy_protocol): When enabled, requires this parameter to be set to `true` for proper internal service routing.
- [internal_router](/configuration/rack-parameters/aws/internal_router): May be affected by DNS resolver choice when routing internal traffic.

## Version Requirements
This feature requires at least Convox rack version `3.21.4`.