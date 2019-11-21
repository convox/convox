# Load Balancing

## Standard Load Balancer

Each Rack contains a built-in HTTP/HTTPS load balancer.

For an app named `myapp` with a `convox.yml` like this:

    services:
      web:
        port: 3000

Convox will automatically set up HTTPS load balancing to this service when it is deployed.

    $ convox services
    SERVICE  DOMAIN                               PORTS
    web      web.myapp.0a1b2c3d4e5f.convox.cloud  443:3000

You can then access the `web` service of this application using `https://web.myapp.0a1b2c3d4e5f.convox.cloud`

### SSL Termination

Convox will automatically configure SSL for the external services of your app using a certificate from
[Lets Encrypt](https://letsencrypt.org/).

> Convox will redirect HTTP requests on port 80 to HTTPS on port 443 using an HTTP 301 redirect.

### End-to-End Encryption

In the example above, a connection to your application would be HTTPS between the user and the Rack's load
balancer and then HTTP between the load balancer and the application.

If you would like this connection to be encrypted all the way to your application you must configure your
application to listen for HTTPS on its defined port and update your `convox.yml`:

    services:
      web:
        port: https:3000

> It is permissible to use a self-signed certificate between the Rack load balancer and your application.

## Custom Load Balancers

If your application needs to expose arbitrary TCP ports or more than one HTTP port, you can configure
custom load balancers.

For a `convox.yml` like this:

    balancers:
      other:
        service: web
        ports:
          5000: 3000
          5001: 3001
    services:
      web:
        port: 3000

Convox will configure a dedicated load balancer for each entry in the `balancers:` section.

    $ convox balancers
    BALANCER  SERVICE  ENDPOINT
    other     web      1.2.3.4

You could then access this application using the following URLs:

* `http://1.2.3.4:5000`
* `tcp://1.2.3.4:5001`

> Note that Convox will not configure SSL termination for ports on a custom load balancer.