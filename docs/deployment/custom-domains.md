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

You will need to alias your custom domain to your Rack's router endpoint. You can find this with `convox rack`:

    $ convox rack
    Name      convox
    Provider  gcp
    Router    router.0a1b2c3d4e5f.convox.cloud
    Status    running
    Version   master

In this example you would set up the following DNS entry:

    myapp.example.org CNAME router.0a1b2c3d4e5f.convox.cloud
