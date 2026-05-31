---
title: "router_type"
description: "The router_type AWS rack parameter selects the Rack ingress controller, either nginx (the default) or contour, an Envoy-based alternative."
slug: router_type
url: /configuration/rack-parameters/aws/router_type
---

# router_type

## Description

The `router_type` parameter selects the ingress controller for the Rack: `nginx` (the default) or `contour` (an Envoy-based alternative).

`nginx` is the long-standing default ingress controller for V3 AWS Racks. `contour` is an opt-in alternative backed by Contour and Envoy. Convox is moving toward a supported ingress controller because nginx upstream free support is ending, and Contour is the first alternative offered. nginx stays installed regardless of which value you choose, so you can switch back at any time.

Changing `router_type` on a Rack that already has running Apps takes every App offline until each one is redeployed. When you switch the controller, your existing Apps do not carry over automatically: they stop serving public traffic the moment the switch applies, and each App stays unreachable externally until you run a deploy for it. Run a separate deploy per App. There is no zero-downtime migration yet. Switching back from `contour` to `nginx` has the same cost: every App must be redeployed again.

Because of this, use `contour` only on brand-new Racks (set it before deploying any Apps, so there is nothing to migrate) or on staging and test Racks where you can take the Apps down and redeploy them. Do not flip a production Rack to `contour` expecting a quick, low-risk trial.

See [Ingress Router](/configuration/ingress-router) for the full migration explanation.

## Default Value

The default value is `nginx`. This default preserves existing behavior: a Rack that does not set `router_type` continues to use the nginx ingress controller exactly as before, with no change to running Apps.

## Use Cases

- Provision a brand-new Rack on the Contour (Envoy) ingress controller by setting `router_type=contour` before deploying any Apps.
- Evaluate Contour on a staging or test Rack where a full redeploy of every App is acceptable.

## Setting the Parameter

```bash
$ convox rack params set router_type=contour -r rackName
```
Setting parameters... OK

## Viewing Current Configuration

```bash
$ convox rack params -r rackName
```

## Additional Information

This parameter is available on AWS Racks only and requires Rack version `3.24.8` or later.

When you change `router_type` on a Rack with running Apps, the CLI prints a warning that every App must be redeployed. Until an App is redeployed it has no route on the new controller and is unreachable externally.

When `router_type=contour`, Contour and Envoy handle both external ingress (the NLB points at the Envoy Pods) and `internalRouter` services behind the private internal NLB. The related Contour and Envoy parameters (`contour_internal_tls`, `contour_cpu_request`, `contour_memory_request`, `envoy_cpu_request`, `envoy_memory_request`) take effect only when `router_type=contour`. They have no effect while the Rack uses `nginx`.

Contour is configured to return uncompressed responses, matching nginx behavior. Apps that want compression handle it themselves.

## See Also

- [Ingress Router](/configuration/ingress-router)
- [contour_internal_tls](/configuration/rack-parameters/aws/contour_internal_tls)
- [contour_memory_request](/configuration/rack-parameters/aws/contour_memory_request)
- [envoy_memory_request](/configuration/rack-parameters/aws/envoy_memory_request)
