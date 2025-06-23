---
title: "Service Discovery"
draft: false
slug: Service Discovery
url: /configuration/service-discovery
---
# Service Discovery

## Load Balancer

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
These external URLs provide public access to your services through the load balancer with automatic SSL termination.

> Note that these [Services](/reference/primitives/app/service) are available to the public internet through the load balancer.

## Internal Services

For internal service-to-service communication within your rack, use the internal DNS names instead of the external load balancer domains. Internal service discovery allows services to communicate directly without routing through the public load balancer.

### Internal Services Configuration

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

The `web` [Service](/reference/primitives/app/service) could reach the internal `auth` [Service](/reference/primitives/app/service) using:

* `http://auth.myrack-myapp.svc.cluster.local:5000` (internal DNS)

> Note that internal services do not receive automatic SSL termination. If you want encrypted communication between internal services, you would need to handle SSL within the [Service](/reference/primitives/app/service) itself or use the internal DNS names which provide the same security boundary.

### Internal DNS Format

Internal services can be accessed using the following DNS format:
```
<serviceName>.<rackName>-<appName>.svc.cluster.local
```

For the example above with a rack named `myrack` and an app named `myapp`, the internal DNS names would be:
- `auth.myrack-myapp.svc.cluster.local`
- `web.myrack-myapp.svc.cluster.local`

### Environment Variables

Convox automatically provides environment variables that make it easy to construct internal service URLs programmatically. Since services typically communicate with other services within the same rack and app, you can use:

```
<serviceName>.${RACK}-${APP}.svc.cluster.local
```

For example, in your application code, you could connect to the auth service using:
```
auth.${RACK}-${APP}.svc.cluster.local:5000
```

Where `RACK` and `APP` are environment variables automatically set by Convox containing your rack name and app name respectively.

### Benefits of Internal Service Discovery

- **Better Performance**: Direct service-to-service communication without load balancer overhead
- **Reduced Latency**: Eliminates external routing hops
- **Cost Efficiency**: Avoids load balancer data transfer costs for internal traffic
- **Security**: Traffic remains within the cluster network
