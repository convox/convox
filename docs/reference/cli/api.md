# api

## api get

Query the Rack API

### Usage

    convox api get <path>

### Examples

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