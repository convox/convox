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

A balancer port is either TCP or UDP; no other protocol is accepted. By default a custom balancer can carry multiple TCP ports or multiple UDP ports, but not a mix of both. Setting `awsLoadBalancerController: true` on the balancer lifts that restriction for TCP and UDP on *different* port numbers. The same port number serving both TCP and UDP is not supported.

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

### The AWS Load Balancer Controller Path

`awsLoadBalancerController` provisions the balancer through the AWS Load Balancer Controller rather than the in-cluster Kubernetes cloud provider. It is supported on AWS racks only; a deploy on any other provider is rejected. There are a few things to know before enabling it.

**It cannot be added to a balancer that already has a load balancer.** Switching an existing balancer over would leave its old load balancer running in your AWS account with nothing in Kubernetes referencing it, so Convox refuses the deploy. A balancer that never received an address, such as one whose mixed TCP and UDP ports were rejected on the default path, has nothing to leave behind, so the flag can be set on it directly. Otherwise migrate:

1. Add a second balancer under a new name with `awsLoadBalancerController: true` and deploy. `convox balancers` now shows both endpoints.
2. Move whatever points at the old endpoint over to the new one.
3. Remove the old balancer entry and deploy. Its load balancer, target groups and node security group rules are cleaned up.

**The endpoint is new.** The controller never adopts an existing load balancer, so the balancer gets a different address.

**The health check follows the first TCP port.** A balancer with UDP ports gets a TCP health check pointed at its first TCP target port, because a TCP probe against a UDP port can never pass. Set `service.beta.kubernetes.io/aws-load-balancer-healthcheck-port` in the balancer's `annotations` to override it, and set it explicitly if the balancer has no TCP port at all.

**The scheme is internet-facing** unless the balancer's own annotations set `service.beta.kubernetes.io/aws-load-balancer-scheme` or `service.beta.kubernetes.io/aws-load-balancer-internal`.

**A `service.beta.kubernetes.io/aws-load-balancer-security-groups` annotation disables `whitelist` enforcement**, because it replaces the security group the controller would otherwise manage for you.

**Targets are registered by pod IP** rather than through a node port. During a rolling deploy the target group can briefly hold only targets that are still registering, so a service with few replicas may drop connections mid-deploy.

**Client IP preservation is always on for UDP targets.** A pod that reaches its own balancer's UDP port from inside the cluster is not reachable that way when it lands on the same node as a target pod.

If a balancer never gets an address and `convox balancers` shows it empty, the most common cause is an annotation that stops the controller from claiming the Service at all, in particular overriding `service.beta.kubernetes.io/aws-load-balancer-nlb-target-type`. The controller declines silently in that case, with no event on the Service.

## See Also

- [Ingress Router](/configuration/ingress-router) for choosing between the nginx and Contour (Envoy) routers
- [Custom Domains](/deployment/custom-domains) for routing custom domains to your services
- [SSL](/deployment/ssl) for configuring SSL certificates
- [Health Checks](/configuration/health-checks) for configuring health check endpoints
