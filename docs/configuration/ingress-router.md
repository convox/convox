---
title: "Ingress Router"
slug: ingress-router
url: /configuration/ingress-router
---

# Ingress Router

Every AWS V3 Rack runs an ingress router that accepts external traffic and routes it to your Services. The ingress controller behind that router is `nginx` by default, and it handles ingress for every App on the Rack without any configuration on your part.

Contour, an Envoy-based ingress controller, is an opt-in alternative. You select it with the `router_type` Rack parameter. When Contour is active, an NLB points at Envoy pods, and Contour handles both external ingress and `internalRouter` Services.

Contour exists because the upstream free support for `nginx` is ending, so Convox is moving toward a supported ingress controller. Contour is the first alternative offered. `nginx` stays installed when you switch to Contour, so you can return to it at any time by setting `router_type` back to `nginx`.

## Choosing a router

The `router_type` parameter accepts two values:

- `nginx` (the default): the existing ingress controller, used on every Rack unless you opt in to Contour.
- `contour`: the Envoy-based ingress controller.

For a NEW Rack, set `router_type=contour` before you deploy any Apps. Every App then comes up on Contour from the start, with nothing to migrate and no downtime.

```bash
$ convox rack params set router_type=contour -r rackName
```

## Switching an existing rack

> **Read this before switching `router_type` on a Rack that already has running Apps.**
>
> Switching `router_type` on a Rack with running Apps takes EVERY App offline until you redeploy it. The instant the switch completes, your Apps stop serving external traffic.
>
> The switch does not move your existing Apps to the new controller automatically. Each App has no route on the new controller until its next deploy, so each App stays unreachable until you redeploy it. Redeploy each App with a deploy command (`convox deploy <app> -r rackName`).
>
> On a Rack with several Apps, or where deploys are slow, this can be a lengthy outage. There is no automatic zero-downtime migration yet; it is planned for a later release.
>
> Switching back to `nginx` costs the same: every App must be redeployed again.

Because of the outage described above, use `router_type=contour` only for brand-new Racks, or for staging and test Racks you can afford to take down and redeploy. Do NOT flip a production Rack to Contour expecting a quick, low-risk trial.

## Internal router TLS

When `router_type=contour`, `internalRouter` Services serve HTTPS using a certificate from the Rack's own self-signed CA by default. This behavior is controlled by the `contour_internal_tls` parameter, which is on by default.

This is skip-verify HTTPS: it provides encryption in transit, not publicly verified trust. It matches the posture of `nginx`-internal, which also serves HTTPS with a non-verified certificate. In-cluster callers either skip verification or trust the Rack CA.

To serve plain HTTP on the internal router and skip the CA and certificate resources entirely, set `contour_internal_tls=false`.

```bash
$ convox rack params set contour_internal_tls=false -r rackName
```

`contour_internal_tls` has no effect on `nginx` Racks.

## Tuning Contour and Envoy resources

Contour runs as a control plane, and Envoy runs as the data plane. Each has its own CPU and memory requests that you can tune independently.

Control-plane requests (Contour):

- [`contour_cpu_request`](/configuration/rack-parameters/aws/contour_cpu_request): CPU request for Contour control-plane pods.
- [`contour_memory_request`](/configuration/rack-parameters/aws/contour_memory_request): memory request for Contour control-plane pods. Worth raising on Racks with many routes.

Data-plane requests (Envoy):

- [`envoy_cpu_request`](/configuration/rack-parameters/aws/envoy_cpu_request): CPU request for Envoy data-plane pods.
- [`envoy_memory_request`](/configuration/rack-parameters/aws/envoy_memory_request): memory request for Envoy data-plane pods.

```bash
$ convox rack params set envoy_cpu_request=200m envoy_memory_request=512Mi -r rackName
```

## See Also

- [`router_type`](/configuration/rack-parameters/aws/router_type)
- [`contour_internal_tls`](/configuration/rack-parameters/aws/contour_internal_tls)
- [`contour_cpu_request`](/configuration/rack-parameters/aws/contour_cpu_request)
- [`contour_memory_request`](/configuration/rack-parameters/aws/contour_memory_request)
- [`envoy_cpu_request`](/configuration/rack-parameters/aws/envoy_cpu_request)
- [`envoy_memory_request`](/configuration/rack-parameters/aws/envoy_memory_request)
- [Load Balancers](/configuration/load-balancers)
- [Rack to Rack](/configuration/rack-to-rack)
