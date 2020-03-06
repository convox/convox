# Balancer

A Balancer accepts incoming traffic and balances it between the [Processes](process.md) of a [Service](service.md).

## Definition

A Balancer is defined in [`convox.yml`](../../../configuration/convox-yml.md).

    balancers:
      ingress:
        service: mqtt
        ports:
          8883: 8883
    services:
      mqtt:
        ports:
          - 8883

### Attributes

| Name      | Required | Description                                                                                                                                                                                            |
| --------- | -------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `ports`   | **yes**  | A map of ports in the format `listen:forward` where `listen` is the port that the balancer will listen on and `forward` is the port that the traffic will be forwarded to on the [Service](service.md) |
| `service` | **yes**  | The name of the service that will receive the traffic                                                                                                                                                  |

## Command Line Interface

### Listing Balancers

    $ convox balancers
    BALANCER  SERVICE  ENDPOINT
    ingress   mqtt     1.2.3.4