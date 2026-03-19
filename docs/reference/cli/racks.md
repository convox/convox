---
title: "racks"
slug: racks
url: /reference/cli/racks
---
# racks

The `convox racks` command lists all Racks accessible from your current Console login, along with their cloud provider and status. Use `convox switch` to change the active Rack.

## racks

List available Racks and their state.

### Usage
```bash
    convox racks
```
### Examples
```bash
    $ convox racks
    NAME               PROVIDER  STATUS
    dev                local     running
    testing            do        running
    acme/production    aws       running
    acme/staging       gcp       running
    integration/test   azure     running
```

## See Also

- [rack](/reference/cli/rack) for managing a specific rack
- [login](/reference/cli/login) for authenticating with a Console