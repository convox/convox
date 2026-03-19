---
title: "proxy"
slug: proxy
url: /reference/cli/proxy
---
# proxy

The `convox proxy` command creates a secure tunnel from your local machine to internal Rack resources. It is commonly used to access databases, caches, or other services that are not publicly exposed. The tunnel listens on localhost and forwards traffic through the Rack.

## proxy

Proxy a connection inside the rack

### Usage
```bash
    convox proxy <[port:]host:hostport> [[port:]host:hostport]...
```
### Examples
```bash
    $ convox proxy i-06d0eaf588c96ee5f:5432
    proxying localhost:5432 to i-06d0eaf588c96ee5f:5432
```

You can find instance IDs with `convox instances`.

## See Also

- [Resources](/reference/primitives/app/resource) for resource configuration