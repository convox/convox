---
title: "balancers"
description: "The convox balancers command lists custom Balancers for an app, the dedicated load balancers that expose non-HTTP TCP and UDP services."
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
### Flags

| Flag | Short | Description |
| ---- | ----- | ----------- |
| `--app` | `-a` | App name |
| `--rack` | `-r` | Rack name |
| `--watch` | | Rerun the command every given number of seconds |

### Examples
```bash
    $ convox balancers
    BALANCER  SERVICE  ENDPOINT
    other     web      1.2.3.4
    custom    worker   (pending)
```

In a terminal, a balancer whose load balancer has no address yet shows `(pending)`. Piped or redirected output has an empty cell instead, and `convox api get /apps/<app>/balancers` returns `"endpoint": ""`. The marker requires CLI version `3.25.9` or later. See [A Balancer With No Endpoint](/configuration/load-balancers#a-balancer-with-no-endpoint).

## See Also

- [Load Balancers](/configuration/load-balancers) for load balancer configuration