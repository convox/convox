---
title: "Custom Domains"
slug: custom-domains
url: /deployment/custom-domains
---
# Custom Domains

Custom domains allow you to route one or more domains to a [Service](/reference/primitives/app/service).

## Definition examples
```yaml
    services:
      simpleweb:
        domain: myapp.example.org
      ...
      complexweb:
        domain: subdomain1.example.org,subdomain2.example.org,somethingelse.test.com
```
Multiple domains should be comma separated. Wildcard domains (e.g., `*.example.org`) are supported when using DNS-01 validation with a pre-generated wildcard certificate. To use a wildcard domain:

1. Configure [DNS-01 validation](/deployment/ssl#advanced-ssl-configuration-lets-encrypt-dns01-challenge-with-route53-aws) (currently available for AWS Route53).
2. Generate the wildcard certificate with `convox certs generate "*.example.org" --issuer letsencrypt`.
3. Reference the certificate ID in your `convox.yml` using the `certificate` attribute (see [SSL](/deployment/ssl#wildcard-certificates-and-reuse)).

Wildcard domains are not supported with the default HTTP-01 automatic certificate flow.

### Dynamic Configuration

You can avoid hardcoding your custom domains in `convox.yml` using
[Environment Interpolation](/configuration/environment#interpolation).
```yaml
    services:
      web:
        domain: ${HOST}
```
```bash
$ convox env set HOST=myapp.example.org,myapp2.example.org
```

## Configuring DNS

You will need to alias your custom domain to your Rack's router endpoint. You can find this with `convox rack`:
```bash
    $ convox rack
    Name      convox
    Provider  gcp
    Router    router.0a1b2c3d4e5f.convox.cloud
    Status    running
    Version   master
```
In this example you would set up the following DNS entry:
```bash
    myapp.example.org CNAME router.0a1b2c3d4e5f.convox.cloud
```

## See Also

- [SSL](/deployment/ssl) for configuring SSL certificates for your domains
- [Load Balancers](/configuration/load-balancers) for advanced load balancer configuration
