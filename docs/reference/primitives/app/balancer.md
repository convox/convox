---
title: "Balancer"
draft: false
slug: Balancer
url: /reference/primitives/app/balancer
---
# Balancer

A Balancer accepts incoming traffic and balances it between the [Processes](/reference/primitives/app/process) of a [Service](/reference/primitives/app/service).

## Definition

A Balancer is defined in [`convox.yml`](/configuration/convox-yml).

```html
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
    whitelist:
      - 192.168.0.0/16
```

### Attributes

| Name           | Required | Description                                                                                                    |
| -------------- | -------- | -------------------------------------------------------------------------------------------------------------- |
| **annotations**| no       | A list of annotation keys and values to populate the metadata for the deployed balancer                        |
| **ports**      | **yes**  | A map of ports in the format **listen:forward** where **listen** is the port that the balancer will listen on and **forward** is the port that the traffic will be forwarded to on the [Service](/reference/primitives/app/service) |
| **service**    | **yes**  | The name of the service that will receive the traffic                                                           |
| **whitelist**  | no       | A list of CIDR ranges from which to limit inbound traffic to this balancer                                      |

## Command Line Interface

### Listing Balancers

```html
$ convox balancers
BALANCER  SERVICE  ENDPOINT
custom    mqtt     1.2.3.4
```

## Configuration Examples

### Configuring TCP Ports

To configure TCP ports on a balancer, you can use the following example:

```html
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

```html
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

### Important Notes

- A custom balancer can only be configured with multiple TCP or multiple UDP ports and redirects, but you cannot have both TCP and UDP on the same balancer.
- Ports configured using `ports:` will never be publicly accessible; all connections must go through the load balancer, which is internet-facing.

### Difference Between `port` and `ports`

- **port**: Used to define the main port that the service will listen on. This port is exposed via the default ingress and is used for primary traffic, including health checks.
- **ports**: Used to define additional ports for service-to-service communication within the cluster. These ports can be exposed using a custom balancer for specific protocols like TCP or UDP.

```html
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

> Note: The main `port` must always be defined, and it will use the default ingress. Health checks go over the port defined as `port:`.

### Example of Configuring an Alternate Health Check Port

You can configure an alternate health check port using the `ports` attribute.

```html
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

> Note: Ports defined with the `ports:` attribute will only be accessible within the cluster and through the configured custom balancer.

For more detailed information on configuring load balancers, refer to the [Load Balancers](/configuration/load-balancers) documentation page.
