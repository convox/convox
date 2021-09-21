---
title: "Service Discovery"
draft: false
slug: Service Discovery
url: /configuration/service-discovery
---
# Service Discovery

## Load Balanced

The load balancer hostname for a particular [Service](/reference/primitives/app/service) can
be found using `convox services`.

Connecting to this hostname will distribute traffic across all processes of a given
[Service](/reference/primitives/app/service).

For an app named `myapp` with a `convox.yml` like this:
```html
    services:
      auth:
        port: 5000
      web:
        port: 3000
```
You would see a `convox services` output similar to this:
```html
    $ convox services
    SERVICE  DOMAIN                                PORTS
    auth     auth.myapp.0a1b2c3d4e5f.convox.cloud  443:5000
    web      web.myapp.0a1b2c3d4e5f.convox.cloud   443:3000
```
The `web` [Service](/reference/primitives/app/service) could reach the `auth`
[Service](/reference/primitives/app/service) using `https://auth.myapp.0a1b2c3d4e5f.convox.cloud:443`

> Note that both of these [Services](/reference/primitives/app/service) are available to the public internet.

### Internal Services

You can make a [Service](/reference/primitives/app/service) accessible only inside the Rack
by setting its `internal` attribute to `true`.

For an app named `myapp` with a `convox.yml` like this:
```html
    services:
      auth:
        internal: true
        port: 5000
      web:
        port: 3000
```
You would see a `convox services` output similar to this:
```html
    $ convox services
    SERVICE  DOMAIN                               PORTS
    auth     auth.myapp.convox.local              5000
    web      web.myapp.0a1b2c3d4e5f.convox.cloud  443:3000
```
The `web` [Service](/reference/primitives/app/service) could reach the `auth` [Service](/reference/primitives/app/service) using the following URL:

* `http://auth.myapp.convox.local:5000`

> Note that the internal port of the `auth` [Service](/reference/primitives/app/service) is not receiving
> automatic SSL termination. If you want this connection to be encrypted you would need to handle SSL
> inside the [Service](/reference/primitives/app/service).

DNS search suffixes are automatically configured for internal hostnames on a Rack. The following URLs would
also work for contacting the `auth` [Service](/reference/primitives/app/service):

* `http://auth:5000` for [Services](/reference/primitives/app/service) on the same app.
* `http://auth.myapp:5000` for other [Apps](/reference/primitives/app) on the same Rack.