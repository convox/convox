---
title: "Load Balancers"
description: "Each Rack includes a built-in HTTPS load balancer that fronts your Services, using an AWS NLB or GCP Load Balancer depending on the cloud provider."
slug: load-balancers
url: /configuration/load-balancers
---
# Load Balancers

## Standard Load Balancer

Each Rack contains a built-in HTTPS load balancer, using AWS NLB or GCP Load Balancer based on the cloud provider.

For an app named `myapp` with a `convox.yml` like this:

```yaml
services:
  web:
    port: 3000
```

Convox will automatically set up HTTPS load balancing to this Service when it is deployed.

```bash
$ convox services
SERVICE  DOMAIN                                PORTS
web      web.myapp.0a1b2c3d4e5f.convox.cloud  443:3000
```

You can then access the `web` Service of this App using `https://web.myapp.0a1b2c3d4e5f.convox.cloud`.

### SSL Termination

Convox will automatically configure SSL for the external Services of your app using a certificate from [Let's Encrypt](https://letsencrypt.org/).

> Convox will redirect HTTP requests on port 80 to HTTPS on port 443 using an HTTP 308 redirect.

### Custom SSL Certificates

To use a custom SSL certificate, you can upload it to your Rack:

```bash
$ convox certs import cert.pem key.pem
```

### Custom Domains

See [Custom Domains](/deployment/custom-domains).

### End-to-End Encryption

In the example above, a connection to your App would be HTTPS between the user and the Rack's load balancer and then HTTP between the load balancer and the App.

If you would like this connection to be encrypted all the way to your App you must configure your App to listen for HTTPS on its defined port and update your `convox.yml`:

```yaml
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

```yaml
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
> The `ports` attribute on this Service defines ports that are not exposed using the default load balancer.

Convox will configure a dedicated load balancer for each entry in the `balancers:` section.

```bash
$ convox balancers
BALANCER  SERVICE  ENDPOINT
custom    web      1.2.3.4
```

You could then access this Service using the following endpoints:

- `tcp://1.2.3.4:5000`
- `tcp://1.2.3.4:6000`

> Convox will not configure SSL termination for ports on a custom [Balancer](/reference/primitives/app/balancer).

## Hybrid Load Balancing

It is possible to combine both of these load balancing types on a single Service.

For a `convox.yml` like this:

```yaml
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

```bash
$ convox services
SERVICE  DOMAIN                                PORTS
web      web.myapp.0a1b2c3d4e5f.convox.cloud  443:4000

$ convox balancers
BALANCER  SERVICE  ENDPOINT
custom    web      1.2.3.4
```

And you could access the Service using the following endpoints:

- `https://web.myapp.0a1b2c3d4e5f.convox.cloud`
- `http://1.2.3.4:6000`
- `tcp://1.2.3.4:6001`

> Port 4000 on this Service is exposed through both the standard and custom load balancers. SSL termination is not provided on the custom [Balancer](/reference/primitives/app/balancer).

## UDP Support

Convox supports the use of UDP protocols for custom load balancers. To expose and use a port with the UDP protocol, configure your `convox.yml` like this:

```yaml
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

A balancer port takes `TCP`, `UDP` or `TCP_UDP`; no other protocol is accepted. By default a custom balancer can carry multiple TCP ports or multiple UDP ports, but not a mix of both, a restriction the in-cluster Kubernetes cloud provider enforces rather than Convox. Setting `awsLoadBalancerController: true` on the balancer lifts that restriction. TCP and UDP on different port numbers requires Rack version `3.25.5` or later. `TCP_UDP`, which serves both protocols on one port number, requires Rack version `3.25.6` or later.

```yaml
balancers:
  custom:
    service: web
    awsLoadBalancerController: true
    ports:
      5000: 5001
      6000:
        protocol: UDP
        port: 6001
services:
  web:
    build: .
    port: 3000
    ports:
      - 5001
      - 6001/udp
```

`protocol: TCP_UDP` serves both protocols on one port number through a single AWS `TCP_UDP` listener and target group. It requires `awsLoadBalancerController: true`, and unlike the other protocols it must set a target port.

```yaml
balancers:
  dns:
    service: resolver
    awsLoadBalancerController: true
    ports:
      53:
        protocol: TCP_UDP
        port: 5300
services:
  resolver:
    build: .
```

A balancer serving both protocols on one port number has to be created that way, and its ports cannot be changed afterwards. See [A Balancer Serving Both Protocols on One Port](#a-balancer-serving-both-protocols-on-one-port).

One manifest entry renders as two Kubernetes ports at the same number, so `convox api get /apps/myapp/balancers` reports two entries for it.

Do not put the balancer's target port in the Service's `port:`. `port:` renders an HTTP readiness probe against the health path, which defaults to `/`, so an HTTP GET against a DNS listener never succeeds and the Processes never become Ready. `port:` also publishes that port on the default ingress with a domain and a certificate. A Service can still have its own `port:` for HTTP, as long as it is not the balancer's target port. A Service's `ports:` list cannot carry the same number twice, so a port a balancer serves as `TCP_UDP` reaches the internal `.local` address for one protocol only, either `ports: [5300]` or `ports: [5300/udp]`.

Protocol values are case-insensitive from Rack version `3.25.5`, so `tcp_udp` and `TCP_UDP` both work, and on an earlier Rack the value must be uppercase. The `awsLoadBalancerController` field name is case-sensitive.

Balancer protocols are validated at build, not at promote, and a build reports every balancer error it finds together:

```text
validation errors:
balancer dns port 53 uses protocol TCP_UDP, which requires awsLoadBalancerController: true
balancer dns port 53 uses protocol TCP_UDP and must set a target port
```

| Manifest | Message |
|----------|---------|
| `protocol: TCP_UDP` without `awsLoadBalancerController: true` | `balancer dns port 53 uses protocol TCP_UDP, which requires awsLoadBalancerController: true` |
| `protocol: TCP_UDP` with no `port:` | `balancer dns port 53 uses protocol TCP_UDP and must set a target port` |
| The same `ports:` key twice on a balancer with `awsLoadBalancerController: true` | `balancer dns declares port 53 more than once, use protocol: TCP_UDP on a single entry to serve both protocols on one port number` |
| Any other protocol value | `balancer dns port 53 has unsupported protocol SCTP` |

A `TCP_UDP` manifest can still reach a promote without the attribute, from an older Release promoted forward or a Build made on a Rack that did not run the check. That promote is rejected instead:

```text
balancer dns: protocol TCP_UDP requires awsLoadBalancerController: true, update convox.yml and build again
```

On an earlier Rack the `awsLoadBalancerController` key is ignored rather than rejected. The balancer is provisioned on the default route, and its mixed TCP and UDP ports never receive an address.

`protocol: TCP_UDP` is rejected rather than ignored on an earlier Rack, and where it surfaces depends on what triggered the deploy.

| Rack downgraded to | With a `TCP_UDP` manifest | Where it surfaces |
|--------------------|---------------------------|-------------------|
| `3.25.5`, with a rebuild | `balancer dns port 53 has unsupported protocol TCP_UDP` | At build |
| `3.25.5`, on a promote, a rollback, `convox apps params set` or `convox apps update` of an existing Release | The manifest is not revalidated, so the value reaches Kubernetes: `Unsupported value: "TCP_UDP": supported values: "SCTP", "TCP", "UDP"`. It keeps failing until a new Build | At deploy |
| `3.25.4` or earlier | The same Kubernetes rejection | At deploy |

Each of those rejects the deploy and leaves the live balancer's Kubernetes object unchanged.

> Do not downgrade a Rack below `3.25.5` while a balancer with `protocol: TCP_UDP` is live. The Rack's AWS Load Balancer Controller reverts to a chart with no `TCP_UDP` support, reconciles the balancer, and drops one of its two listeners with no deploy, no error and no event. The Kubernetes object is untouched, so nothing reports it. Remove the balancer first.

## The awsLoadBalancerController Attribute

`awsLoadBalancerController` provisions the balancer through the AWS Load Balancer Controller rather than through the in-cluster Kubernetes cloud provider. It requires Rack version `3.25.5` or later, and AWS Racks only. Serving both protocols on one port number with `protocol: TCP_UDP` requires Rack version `3.25.6` or later. A deploy on any other provider is rejected:

```text
balancer custom: awsLoadBalancerController is only supported on AWS racks
```

### Switching an Existing Balancer

The attribute cannot be added to a balancer that already has a load balancer. Switching an existing balancer over would leave its old load balancer running in your AWS account with nothing in Kubernetes referencing it, so Convox rejects the promote:

```text
balancer custom cannot be switched to the AWS Load Balancer Controller in place, because the existing load balancer would be left running in your AWS account. Add a new balancer with awsLoadBalancerController: true, move traffic to it, then remove this one
```

The same rejection fires when the balancer's own `annotations` set `service.beta.kubernetes.io/aws-load-balancer-type` to `external` or `nlb-ip`, which hands the Service to the controller the way the attribute does.

A balancer that never received an address, such as one whose mixed TCP and UDP ports were rejected on the default route, is not affected, so the attribute can be set on it directly. A `TCP_UDP` port never reaches that state, because a build without `awsLoadBalancerController: true` is rejected and no Service is created. Otherwise migrate:

1. Add a second balancer under a new name with `awsLoadBalancerController: true` and deploy. `convox balancers` now shows both endpoints.
2. Move whatever points at the old endpoint over to the new one.
3. Remove the old balancer entry and deploy. Its load balancer, target groups and node security group rules are cleaned up.

### A Balancer Serving Both Protocols on One Port

A balancer serving both protocols on one port number has to be created that way. On `3.25.6` a `TCP_UDP` port cannot be added to a balancer that already exists, and once a balancer has a `TCP_UDP` port its ports cannot be changed: adding a port, removing a port, changing a target port, changing a protocol, removing `awsLoadBalancerController: true`, and reordering the entries under `ports:` are all refused. Convox refuses these because it cannot apply the change safely, not because AWS or the controller rejects them.

This is a separate rule from the ownership check above, with a different trigger and no exemption for a balancer that has no address yet. It fires only when a same-port TCP and UDP pair exists either in `convox.yml` or on the live balancer. An `awsLoadBalancerController: true` balancer with no `TCP_UDP` port keeps full port mutability, reordering included.

```text
balancer dns: TCP and UDP on one port number can only be set up on a new balancer, and its ports cannot be changed afterwards. It is currently serving 53/TCP->5300, 53/UDP->5300. Restore this balancer's previous ports in convox.yml to deploy it again, or replace it: add a second balancer with the ports you want and move traffic to it, or remove this one, deploy, then add it back. Replacing it gives a new address
```

The ports the message lists are the live Service's ports in their live order, which is what to restore.

- The refusal fails that deploy and nothing else. The App's current Release is unaffected, so `convox apps params set`, `convox apps update` and a promote of the current Release all keep working.
- **Adding UDP to a port a balancer already serves as plain TCP is refused.** Putting `protocol: TCP_UDP` on an existing DNS-over-TCP balancer is this case, and the message lists only the one live port, so it reads shorter than expected.
- **Removing the balancer's block from `convox.yml` always works.** It deletes the balancer and its load balancer, and it is never refused, because the rule only looks at balancers the manifest still declares. Removing a balancer and adding it back under the same name is two deploys, not one.
- **A rollback ends two different ways.** A rollback to a Release that has the balancer at different ports is refused with the same message. A rollback to a Release from before the balancer existed succeeds and deletes the balancer and its load balancer, because the balancer is absent from that manifest.
- Removing `awsLoadBalancerController: true` while a `TCP_UDP` pair is live is refused with the same message. Reverting the commit that gave an App its `TCP_UDP` balancer is that edit.
- Renaming a balancer is not refused and deletes its load balancer, which is true of every balancer.

### What Changes

| Aspect | Behavior | Determined by |
|--------|----------|---------------|
| Endpoint | The controller never adopts an existing load balancer, so the balancer gets a new address. | Controller |
| Health check | A TCP probe against the balancer's first TCP target port. A balancer whose only UDP-bearing ports are `TCP_UDP` gets no health check annotations, so each target group is probed over TCP on its own port. | Convox |
| Scheme | `internet-facing`, unless the balancer's own annotations set `service.beta.kubernetes.io/aws-load-balancer-scheme` or `service.beta.kubernetes.io/aws-load-balancer-internal`. | Convox |
| Targets | Registered by Pod IP rather than through a node port. | Controller |
| Client IP on UDP | Always preserved. | AWS |
| `whitelist` | Not enforced when the balancer's annotations set `service.beta.kubernetes.io/aws-load-balancer-security-groups`. | Controller |

Rows marked Controller or AWS are AWS Load Balancer Controller and AWS behavior, not Convox behavior.

A balancer with UDP ports gets a TCP health check because a TCP probe against a UDP port can never pass. Set `service.beta.kubernetes.io/aws-load-balancer-healthcheck-port` in the balancer's `annotations` to point the probe somewhere else. A balancer with UDP ports and no TCP port has no port to probe, and the build is rejected unless the annotations set either that key or `service.beta.kubernetes.io/aws-load-balancer-healthcheck-protocol`. A `TCP_UDP` port counts as a TCP port, so that rule does not fire for a balancer whose only port is `TCP_UDP`.

A `TCP_UDP` target group is probed over TCP on the port both protocols share, so a container that accepts only UDP there reports unhealthy. Traffic usually keeps flowing, because an NLB forwards to every target once they all fail their check. What is lost is health reporting, alarms, and zonal DNS behavior when only some targets fail. Point the probe at a port the container does accept TCP on with `service.beta.kubernetes.io/aws-load-balancer-healthcheck-port`, which does not have to be a port the balancer carries.

Convox generates `service.beta.kubernetes.io/aws-load-balancer-enable-tcp-udp-listener` on a balancer that has a `TCP_UDP` port, and it is the one generated annotation a balancer's own `annotations` cannot override.

Pod IP targets change how a rolling deploy behaves. The target group can briefly hold only targets that are still registering, so a Service with few replicas may drop connections mid-deploy.

Client IP preservation on UDP targets means a Pod that reaches its own balancer's UDP port from inside the cluster is not reachable that way when it lands on the same node as a target Pod.

### A Balancer With No Endpoint

If a balancer never gets an address and `convox balancers` shows it empty, the most common cause is an annotation that stops the controller from claiming the Service, in particular overriding `service.beta.kubernetes.io/aws-load-balancer-nlb-target-type`. The controller declines with no event on the Service, so nothing in Convox reports the cause.

## See Also

- [Ingress Router](/configuration/ingress-router) for choosing between the nginx and Contour (Envoy) routers
- [Custom Domains](/deployment/custom-domains) for routing custom domains to your services
- [SSL](/deployment/ssl) for configuring SSL certificates
- [Health Checks](/configuration/health-checks) for configuring health check endpoints
