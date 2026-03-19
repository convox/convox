---
title: "Service Discovery"
slug: service-discovery
url: /configuration/service-discovery
---
# Service Discovery

## Load Balancer

The load balancer hostname for a particular [Service](/reference/primitives/app/service) can
be found using `convox services`.

Connecting to this hostname will distribute traffic across all processes of a given
[Service](/reference/primitives/app/service).

For an app named `myapp` with a `convox.yml` like this:
```yaml
services:
  auth:
    port: 5000
  web:
    port: 3000
```
You would see a `convox services` output similar to this:
```bash
    $ convox services
    SERVICE  DOMAIN                                PORTS
    auth     auth.myapp.0a1b2c3d4e5f.convox.cloud  443:5000
    web      web.myapp.0a1b2c3d4e5f.convox.cloud   443:3000
```
The `web` [Service](/reference/primitives/app/service) could reach the `auth`
[Service](/reference/primitives/app/service) using `https://auth.myapp.0a1b2c3d4e5f.convox.cloud:443`

> Both of these [Services](/reference/primitives/app/service) are available to the public internet.

### Internal Services

You can make a [Service](/reference/primitives/app/service) accessible only inside the Rack
by setting its `internal` attribute to `true`.

For an app named `myapp` with a `convox.yml` like this:
```yaml
services:
  auth:
    internal: true
    port: 5000
  web:
    port: 3000
```
You would see a `convox services` output similar to this:
```bash
    $ convox services
    SERVICE  DOMAIN                               PORTS
    auth     auth.myapp.convox.local              5000
    web      web.myapp.0a1b2c3d4e5f.convox.cloud  443:3000
```
The `web` [Service](/reference/primitives/app/service) could reach the `auth` [Service](/reference/primitives/app/service) using the following URL:

* `http://auth.myapp.convox.local:5000`

> The internal port of the `auth` [Service](/reference/primitives/app/service) is not receiving
> automatic SSL termination. If you want this connection to be encrypted you would need to handle SSL
> inside the [Service](/reference/primitives/app/service).
