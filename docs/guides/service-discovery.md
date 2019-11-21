# Service Discovery

## Load Balanced

The load balancer hostname for a particular service can be found using `convox services`.

Connecting to this hostname will distribute traffic across all processes of a given service.

For an app named `myapp` with a `convox.yml` like this:

    services:
      auth:
        port: 5000
      web:
        port: 3000

You would see a `convox services` output similar to this:

    $ convox services
    SERVICE  DOMAIN                                PORTS
    auth     auth.myapp.0a1b2c3d4e5f.convox.cloud  443:5000
    web      web.myapp.0a1b2c3d4e5f.convox.cloud   443:3000

The `web` service could reach the `auth` service using `https://auth.myapp.0a1b2c3d4e5f.convox.cloud:443`

> Note that both of these services are available to the public internet.

#### Internal Services

You can make a service accessible only inside the Rack by setting its `internal` attribute to `true`.

For an app named `myapp` with a `convox.yml` like this:

    services:
      auth:
        internal: true
        port: 5000
      web:
        port: 3000

You would see a `convox services` output similar to this:

    $ convox services
    SERVICE  DOMAIN                               PORTS
    auth     auth.convox-myapp.svc.cluster.local  5000:5000
    web      web.myapp.0a1b2c3d4e5f.convox.cloud  443:3000

The `web` service could reach the `auth` service using `http://auth.convox-myapp.svc.cluster.local:5000`

> Note that the internal `auth` service is no longer receiving automatic HTTPS termination. If you want this
> connection to be encrypted you would need to terminate HTTPS inside your service.

DNS search suffixes are automatically configured for internal hostnames on a Rack. The following URLs would
also work for contacting the `auth` service:

* `http://auth:5000` for services on the same app.
* `http://auth.convox-myapp:5000` for other apps on the same Rack.

> The `convox` portion of the internal hostnames in the examples above is the name of the Rack.
> You can find the name of a Rack using `convox rack`.

## Individual Processes

TBD