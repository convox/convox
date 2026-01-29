---
title: "api"
draft: false
slug: api
url: /reference/cli/api
---
# api

## api get

Query [primitives](/reference/primitives/) in JSON format using the Rack API 

### Usage
```html
    convox api get <path>
```

App primitives
```html
    convox api get /apps/
    convox api get /apps/<app_name>/balancers
    convox api get /apps/<app_name>/builds 
    convox api get /apps/<app_name>/processes
    convox api get /apps/<app_name>/releases 
    convox api get /apps/<app_name>/resources
    convox api get /apps/<app_name>/services
```

Rack primitives
```html
    convox api get /instances
    convox api get /registries
```
### Examples
```html
    $ convox api get /apps
    [
      {
        "generation": "3",
        "locked": false,
        "name": "myapp",
        "release": "RABCDEFGHI",
        "router", "0a1b2c3d4e5f.convox.cloud",
        "status": "running"
      }
    ]
```
