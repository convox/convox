---
title: "proxy"
draft: false
slug: proxy
url: /reference/cli/proxy
---
# proxy

## proxy

Proxy a connection inside the rack

### Usage
```html
    convox proxy <[port:]host:hostport> [[port:]host:hostport]...
```
### Examples
```html
    $ convox proxy i-06d0eaf588c96ee5f:5432
    proxying localhost:5432 to i-06d0eaf588c96ee5f:5432
```