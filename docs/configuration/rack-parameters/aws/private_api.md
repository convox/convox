---
title: "private_api"
draft: false
slug: private_api
url: /configuration/rack-parameters/aws/private_api
---

# private_api

## Description
The `private_api` parameter disables the public ingress used by the Convox Rack API. When enabled, the API no longer receives a public load balancer and must be accessed through a private network path (for example, VPC peering, VPN, or Tailscale).

## Default Value
The default value for `private_api` is `false`.

## Use Cases
- **Harden rack control-plane access** by ensuring only private network clients can reach the API.
- **Pair with zero-trust access tools** such as Tailscale or private bastions before enabling the `disable_public_access` EKS setting.

## Setting Parameters
```html
$ convox rack params set private_api=true -r rackName
Updating parameters... OK
```

## Additional Information
Once `private_api` is enabled, automation and engineers must reach the Rack API through a private network endpoint (for example, Kubernetes internal DNS or a Tailscale hostname). Ensure any tooling that previously relied on the public `api.<rack>.convox.cloud` host is updated accordingly.
