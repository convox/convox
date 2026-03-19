---
title: "balancers"
slug: balancers
url: /reference/cli/balancers
---
# balancers

Custom [Balancers](/reference/primitives/app/balancer) expose non-HTTP TCP and UDP services through dedicated load balancers.

## balancers

List balancers for an app

### Usage
```bash
    convox balancers
```
### Examples
```bash
    $ convox balancers
    BALANCER  SERVICE  ENDPOINT
    other     web      1.2.3.4
```

## See Also

- [Load Balancers](/configuration/load-balancers) for load balancer configuration