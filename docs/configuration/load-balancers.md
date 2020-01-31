# Load Balancers

## Standard Load Balancer

Each Rack contains a built-in HTTPS load balancer.

For an app named `myapp` with a `convox.yml` like this:

    services:
      web:
        port: 3000

Convox will automatically set up HTTPS load balancing to this Service when it is deployed.

    $ convox services
    SERVICE  DOMAIN                               PORTS
    web      web.myapp.0a1b2c3d4e5f.convox.cloud  443:3000

You can then access the `web` Service of this App using `https://web.myapp.0a1b2c3d4e5f.convox.cloud`

### SSL Termination

Convox will automatically configure SSL for the external Services of your app using a certificate from
[Lets Encrypt](https://letsencrypt.org/).

> Convox will redirect HTTP requests on port 80 to HTTPS on port 443 using an HTTP 301 redirect.

### Custom Domains

You can configure a custom domain for your Service in `convox.yml`:

    services:
      web:
        domain: myapp.example.org
        port: 3000

#### Dynamic Configuration

You can make your custom domain configurable per-deployment using environment interpolation:

    services:
      web:
        domain: ${DOMAIN}

In this example  your App would use a standard Rack hostname by default, but could be
configured to use a custom domain with the `DOMAIN` environment variable:

    $ convox env set DOMAIN=myapp-staging.example.org -a myapp-staging
    $ convox env set DOMAIN=myapp.example.org -a myapp-production

#### DNS Configuration

You will need to alias your custom domain to your Rack's router endpoint. You can find this with `convox rack`:

    $ convox rack
    Name      convox
    Provider  gcp
    Router    router.0a1b2c3d4e5f.convox.cloud
    Status    running
    Version   master

In this example you would set up the following DNS entry:

    myapp.example.org CNAME 0a1b2c3d4e5f.convox.cloud

### End-to-End Encryption

In the example above, a connection to your App would be HTTPS between the user and the Rack's load
balancer and then HTTP between the load balancer and the App.

If you would like this connection to be encrypted all the way to your App you must configure your
App to listen for HTTPS on its defined port and update your `convox.yml`:

    services:
      web:
        port: https:3000

> It is permissible to use a self-signed certificate between the Rack load balancer and your App.

## Custom Load Balancers

If your App needs to expose arbitrary TCP ports to the outside world, you can configure custom [Balancers](../reference/app/primitives/balancer.md).

For a `convox.yml` like this:

    balancers:
      other:
        service: web
        ports:
          5000: 3000
          5001: 3001
    services:
      web:
        ports:
          - 3000
          - 3001

> Note the use of the `ports` attribute on this Service. Ports defined using `ports` are not exposed using
> the default load balancer.

Convox will configure a dedicated load balancer for each entry in the `balancers:` section.

    $ convox balancers
    BALANCER  SERVICE  ENDPOINT
    other     web      1.2.3.4

You could then access this Service using the following endpoints:

* `tcp://1.2.3.4:5000`
* `tcp://1.2.3.4:5001`
* `tcp://1.2.3.4:5002`

> Note that Convox will not configure SSL termination for ports on a custom [Balancer](../reference/app/primitives/balancer.md).

## Hybrid Load Balancing

It is possible to combine both of these load balancing types on a single Service.

For a `convox.yml` like this:

    balancers:
      other:
        service: web
        ports:
          6000: 4000
          6001: 4001
    services:
      web:
        port: 4000
        ports:
          - 4001

You would see the following at the CLI:

    $ convox services
    SERVICE  DOMAIN                               PORTS
    web      web.myapp.0a1b2c3d4e5f.convox.cloud  443:4000

    $ convox balancers
    BALANCER  SERVICE  ENDPOINT
    other     web      1.2.3.4

And you could access the Service using the following endpoints:

* `https://web.myapp.0a1b2c3d4e5f.convox.cloud`
* `http://1.2.3.4:6000`
* `tcp://1.2.4.5:6001`

> Note that port 4000 on this Service is exposed through both the standard and custom load balancers.
> SSL termination is not provided on the custom [Balancer](../reference/app/primitives/balancer.md).