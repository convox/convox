# Custom Domains

Custom domains allow you to route one or more domains to a [Service](../reference/primitives/app/service.md).

## Definition

    services:
      web:
        domain: myapp.example.org

### Wildcard Domains

    services:
      web:
        domain: "*.example.org"

> YAML requires strings beginning with `*` to be enclosed in quotes.

### Dynamic Configuration

You can avoid hardcoding your custom domains in `convox.yml` using
[Environment Interpolation](../configuration/environment#interpolation).

    services:
      web:
        domain: ${HOST}

```
$ convox env set HOST=myapp.example.org,myapp2.example.org
```

## Configuring DNS

    $ convox rack
    Name      production
    Provider  aws
    Router    router.0a1b2c3d4e5f.convox.cloud
    Status    running

Configure your DNS to point your custom domain as a `CNAME` to the `Router` for
your [Rack](../reference/primitives/rack).