---
title: "Load Balancers"
draft: false
slug: Load Balancers
url: /configuration/load-balancers
---
# Load Balancers

## Standard Load Balancer

Each Rack contains a built-in HTTPS load balancer, using AWS NLB or GCP Load Balancer based on the cloud provider.

For an app named `myapp` with a `convox.yml` like this:

```html
services:
  web:
    port: 3000
```

Convox will automatically set up HTTPS load balancing to this Service when it is deployed.

```html
$ convox services
SERVICE  DOMAIN                               PORTS
web      web.myapp.0a1b2c3d4e5f.convox.cloud  443:3000
```

You can then access the `web` Service of this App using `https://web.myapp.0a1b2c3d4e5f.convox.cloud`.

### SSL Termination

Convox will automatically configure SSL for the external Services of your app using a certificate from [Let's Encrypt](https://letsencrypt.org/).

> Convox will redirect HTTP requests on port 80 to HTTPS on port 443 using an HTTP 301 redirect.

### Custom SSL Certificates

To use a custom SSL certificate, you can upload it to your Rack:

```html
$ convox certs upload -a myapp cert.pem key.pem
```

### Custom Domains

See [Custom Domains](/deployment/custom-domains).

### End-to-End Encryption

In the example above, a connection to your App would be HTTPS between the user and the Rack's load balancer and then HTTP between the load balancer and the App.

If you would like this connection to be encrypted all the way to your App you must configure your App to listen for HTTPS on its defined port and update your `convox.yml`:

```html
services:
  web:
    port: https:3000
```

> It is permissible to use a self-signed certificate between the Rack load balancer and your App.

## Difference Between `port` and `ports`

- **`port:`** Defines the main port and protocol for the service. It is publicly accessible and typically uses HTTPS. Health checks are performed over this port.
- **`ports:`** Defines additional ports and protocols for the service, which can include TCP or UDP. These ports are used for internal communication within the Rack.

### Public Accessibility

Ports configured using `ports:` will not be publicly accessible. All external connections must go through the load balancer, which is internet-facing.

## Custom Load Balancers

If your App needs to expose arbitrary TCP or UDP ports to the outside world, you can configure custom [Balancers](/reference/primitives/app/balancer).

For a `convox.yml` like this:

```html
balancers:
  custom:
    service: web
    ports:
      5000: 5001
      6000: 6001
services:
  web:
    port: 3000
    ports:
      - 5001
      - 6001
```
> Note the use of the `ports` attribute on this Service. Ports defined using `ports` are not exposed using the default load balancer.

Convox will configure a dedicated load balancer for each entry in the `balancers:` section.

```html
$ convox balancers
BALANCER  SERVICE  ENDPOINT
custom    web      1.2.3.4
```

You could then access this Service using the following endpoints:

* `tcp://1.2.3.4:5000`
* `tcp://1.2.3.4:5001`

> Note that Convox will not configure SSL termination for ports on a custom [Balancer](/reference/primitives/app/balancer).

## Hybrid Load Balancing

It is possible to combine both of these load balancing types on a single Service.

For a `convox.yml` like this:

```html
balancers:
  custom:
    service: web
    ports:
      6000: 4000
      6001: 4001
services:
  web:
    port: 4000
    ports:
      - 4001
```

You would see the following at the CLI:

```html
$ convox services
SERVICE  DOMAIN                               PORTS
web      web.myapp.0a1b2c3d4e5f.convox.cloud  443:4000

$ convox balancers
BALANCER  SERVICE  ENDPOINT
custom    web      1.2.3.4
```

And you could access the Service using the following endpoints:

* `https://web.myapp.0a1b2c3d4e5f.convox.cloud`
* `http://1.2.3.4:6000`
* `tcp://1.2.4.5:6001`

> Note that port 4000 on this Service is exposed through both the standard and custom load balancers. SSL termination is not provided on the custom [Balancer](/reference/primitives/app/balancer).

## UDP Support

Convox supports the use of UDP protocols for custom load balancers. To expose and use a port with the UDP protocol, configure your `convox.yml` like this:

```html
balancers:
  custom:
    annotations:
      - test.annotation.org/foo=bar
    service: web
    ports:
      5000:
        protocol: UDP
        port: 3001
services:
  web:
    domain: ${HOST}
    build: .
    port: 3000
    ports:
      - 3001/udp
```

> **Note:** While explicitly declaring TCP like this using `ports` with the protocol is valid, the simpler syntax is recommended for TCP configurations:

### Custom Balancer Protocols

Custom balancers can only be configured with multiple TCP or multiple UDP ports and redirects. You cannot mix TCP and UDP ports on the same balancer.





