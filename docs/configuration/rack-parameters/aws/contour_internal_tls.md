---
title: "contour_internal_tls"
slug: contour_internal_tls
url: /configuration/rack-parameters/aws/contour_internal_tls
---

# contour_internal_tls

## Description

The `contour_internal_tls` parameter controls whether your `internalRouter` Services serve HTTPS when the Rack uses the Contour (Envoy) ingress router (`router_type=contour`).

When set to `true` (the default), `internalRouter` Services serve HTTPS using a certificate issued from a per-Rack self-signed certificate authority (CA). The Rack provisions this CA and the matching certificate automatically when Contour is active, so in-cluster callers reach internal Services over an encrypted connection on the private network.

When set to `false`, `internalRouter` Services serve plain HTTP, and the Rack does not create the self-signed CA or certificate resources.

This parameter is AWS only and applies only when `router_type=contour`. It has no effect on Racks using the nginx router, and no effect on non-AWS Racks, regardless of its value.

## Default Value

The default value is `true`. With Contour active, `internalRouter` Services serve HTTPS from the per-Rack self-signed CA. This default preserves existing behavior: the internal router serves HTTPS using a non-verified certificate, the same posture provided by the nginx internal router.

## Use Cases

- Keep encrypted traffic between in-cluster callers and `internalRouter` Services on a Contour Rack (the default).
- Serve internal traffic over plain HTTP and skip the CA and certificate resources by setting the value to `false`.

## Setting the Parameter

```bash
$ convox rack params set contour_internal_tls=false -r rackName
```
Setting parameters... OK

## Viewing Current Configuration

```bash
$ convox rack params -r rackName
```

## Additional Information

The internal router sits behind a private NLB. Public ACME (HTTP-01) certificates cannot be issued for a private domain, so the Rack uses a self-signed CA instead. When `contour_internal_tls=true`, the Rack provisions a self-signed CA `ClusterIssuer` through cert-manager and issues a certificate that matches the internal hostname.

The trust posture is skip-verify HTTPS: traffic is encrypted in transit, but the certificate is not publicly verified trust. In-cluster callers either skip verification or trust the Rack CA. This mirrors the nginx internal router, which already serves HTTPS with a non-verified certificate, so clients that did not verify the prior nginx certificate continue to skip-verify the Contour certificate.

Setting `contour_internal_tls=false` removes the CA and certificate resources and serves `internalRouter` Services over plain HTTP. Choose this only when you do not want the added CA and certificate resources and accept unencrypted internal traffic.

## See Also

- [Ingress Router](/configuration/ingress-router)
- [router_type](/configuration/rack-parameters/aws/router_type)
- [contour_cpu_request](/configuration/rack-parameters/aws/contour_cpu_request)
- [contour_memory_request](/configuration/rack-parameters/aws/contour_memory_request)
