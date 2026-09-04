---
title: "Balancer"
description: "A Balancer accepts incoming TCP or UDP traffic and balances it across the Processes of a Service, configured with listen-to-forward port maps in convox.yml."
slug: balancer
url: /reference/primitives/app/balancer
---
# Balancer

A Balancer accepts incoming traffic and balances it between the [Processes](/reference/primitives/app/process) of a [Service](/reference/primitives/app/service).

## Balancer Definition

A Balancer is defined in [`convox.yml`](/configuration/convox-yml).

```yaml
balancers:
  custom:
    annotations:
    - test.annotation.org/value=foobar
    service: mqtt
    ports:
      8883: 8883
services:
  mqtt:
    ports:
      - 8883
    whitelist: 192.168.0.0/16
```

### Attributes

| Name           | Required | Description                                                                                                    |
| -------------- | -------- | -------------------------------------------------------------------------------------------------------------- |
| **annotations**| no       | A list of annotation keys and values to populate the metadata for the deployed balancer                        |
| **awsLoadBalancerController** | no | Provision this balancer through the AWS Load Balancer Controller, which allows TCP and UDP ports on the same balancer. TCP and UDP on different port numbers requires Rack version 3.25.5 or later. `protocol: TCP_UDP`, which serves both protocols on one port number, requires Rack version 3.25.6 or later. AWS Racks only. The attribute cannot be enabled on a balancer that already has a load balancer. Separately, a balancer serving both protocols on one port number must be created that way, and its ports cannot be changed afterwards. See [Load Balancers](/configuration/load-balancers) |
| **ports**      | **yes**  | A map of ports in the format **listen:forward** where **listen** is the port that the balancer will listen on and **forward** is the port that the traffic will be forwarded to on the [Service](/reference/primitives/app/service). A port can also take a long form that sets `protocol:` (`TCP`, `UDP` or `TCP_UDP`) and `port:` for the target. `TCP_UDP` requires `awsLoadBalancerController: true` and must set a target port. The other protocols default the target to the listen port |
| **service**    | **yes**  | The name of the service that will receive the traffic                                                           |
| **whitelist**  | no       | A list of CIDR ranges from which to limit inbound traffic to this balancer                                      |

## Command Line Interface

### Listing Balancers

```bash
$ convox balancers
BALANCER  SERVICE  ENDPOINT
custom    mqtt     1.2.3.4
```

## Configuration Examples

### Configuring TCP Ports

To configure TCP ports on a balancer, you can use the following example:

```yaml
balancers:
  custom:
    annotations:
      - test.annotation.org/value=foobar
    service: web
    ports:
      5000: 3001
      5002: 3002
services:
  web:
    domain: ${HOST}
    build: .
    port: 3000
    ports:
      - 3001
      - 3002
```

### Configuring UDP Ports

To configure UDP ports on a balancer, specify the protocol explicitly for UDP ports. The default protocol is TCP, so it does not need to be specified for TCP ports.

Protocol values are case-insensitive from Rack version 3.25.5. On an earlier Rack the value must be uppercase.

```yaml
balancers:
  custom:
    annotations:
      - test.annotation.org/value=foobar
    service: web
    ports:
      5000:
        protocol: UDP
        port: 3001
      5002:
        protocol: UDP
        port: 3002
services:
  web:
    domain: ${HOST}
    build: .
    port: 3000
    ports:
      - 3001/udp
      - 3002/udp
```

### Configuring a Port for Both TCP and UDP

`protocol: TCP_UDP` serves both protocols on one port number through a single AWS `TCP_UDP` listener and target group. It requires `awsLoadBalancerController: true` and Rack version `3.25.6` or later, and unlike the other protocols it must set a target port.

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

Do not use the balancer's target port as the Service's `port:`. `port:` renders an HTTP readiness probe against the health path, which defaults to `/`, so a Process serving DNS on that port never becomes Ready and the deploy fails. `port:` also publishes the port on the default ingress with a domain and a certificate. A Service can still declare its own `port:` for HTTP, as long as it is not a port the balancer targets.

A Service's `ports:` list cannot carry the same number twice, so a port a balancer serves as `TCP_UDP` reaches the internal `.local` address for one protocol only, either `ports: [5300]` or `ports: [5300/udp]`.

A balancer serving both protocols on one port number must be created that way, and its ports cannot be changed afterwards. See [Load Balancers](/configuration/load-balancers) for the migration and downgrade rules.

### Important Notes

- A custom balancer can only be configured with multiple TCP or multiple UDP ports and redirects. To mix TCP and UDP ports on one balancer, set `awsLoadBalancerController: true`. From Rack version `3.25.6` one port number can serve both protocols with `protocol: TCP_UDP`.
- Ports configured using `ports:` will never be publicly accessible; all connections must go through the load balancer, which is internet-facing.

### Difference Between `port` and `ports`

- **port**: Used to define the main port that the service will listen on. This port is exposed via the default ingress and is used for primary traffic, including health checks.
- **ports**: Used to define additional ports for service-to-service communication within the cluster. These ports can be exposed using a custom balancer for specific protocols like TCP or UDP.

```yaml
services:
  web:
    domain: ${HOST}
    build: .
    port: 3000
    ports:
      - 3001/udp
      - 3002
```

By using the `ports` attribute, you can configure additional ports with specific protocols on both the Kubernetes service and pod levels.

> The main `port` must always be defined, and it will use the default ingress. Health checks go over the port defined as `port:`.

### Example of Configuring an Alternate Health Check Port

You can configure an alternate health check port using the `ports` attribute.

```yaml
balancers:
  custom:
    annotations:
      - test.annotation.org/foo=bar
    service: web
    ports:
      5000: 3001
      5002: 3002
services:
  web:
    domain: ${HOST}
    build: .
    port: 3000
    ports:
      - 3001
      - 3002
```

In this configuration, the main traffic goes through port 3000, while additional service communication uses ports 3001 and 3002.

> Ports defined with the `ports:` attribute will only be accessible within the cluster and through the configured custom balancer.

For more detailed information on configuring load balancers, refer to the [Load Balancers](/configuration/load-balancers) documentation page.
