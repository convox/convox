---
title: "racks"
description: "The convox racks command lists the racks from your Console login and the local racks the CLI keeps, with each rack's provider and status."
slug: racks
url: /reference/cli/racks
---
# racks

The `convox racks` command lists the Racks accessible from your current Console login and the Racks in the CLI's `racks` settings directory: Racks installed from this machine with Terraform, and Racks saved there by URL. With CLI 3.25.12 or later, a symlink to a Rack directory there is listed the same as the directory. Each Rack is shown with its provider and status. Use `convox switch` to change the active Rack.

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