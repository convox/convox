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
      ingress:
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

| Name        | Required | Description                                                                                                                                                                                            |
| ----------- | -------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| **ports**     | **yes**  | A map of ports in the format **listen:forward** where **listen** is the port that the balancer will listen on and **forward** is the port that the traffic will be forwarded to on the [Service](/reference/primitives/app/service) |
| **service**   | **yes**  | The name of the service that will receive the traffic                                                                                                                                                  |
| **whitelist** | no       | A list of CIDR ranges from which to limit inbound traffic to this balancer                                                                                                                             |

## Command Line Interface

### Listing Balancers
```html
    $ convox balancers
    BALANCER  SERVICE  ENDPOINT
    ingress   mqtt     1.2.3.4
```