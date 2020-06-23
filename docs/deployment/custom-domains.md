# Custom Domains

Custom domains allow you to route one or more domains to a [Service](../reference/primitives/app/service.md).

## Definition examples

    services:
      simpleweb:
        domain: myapp.example.org
      ...
      complexweb:
        domain: subdomain1.example.org,subdomain2.example.org,somethingelse.test.com

Multiple domains should be comma separated.  Due to limitations in the LetsEncrypt validation method for SSL certificates, wildcard domains are not currently supported.

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
