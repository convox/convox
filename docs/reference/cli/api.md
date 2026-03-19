---
title: "api"
slug: api
url: /reference/cli/api
---
# api

The `convox api` command provides low-level access to the Convox Rack API. It is useful for debugging, scripting, or querying API endpoints that are not directly exposed by other CLI commands.

## api get

Query the Rack API

### Usage
```bash
    convox api get <path>
```
### Examples
```bash
    $ convox api get /apps
    [
      {
        "generation": "3",
        "locked": false,
        "name": "myapp",
        "release": "RABCDEFGHI",
        "router": "0a1b2c3d4e5f.convox.cloud",
        "status": "running"
      }
    ]
```