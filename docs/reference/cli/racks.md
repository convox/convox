---
title: "racks"
draft: false
slug: racks
url: /reference/cli/racks
---
# racks

## racks

List available Racks and their state.

### Usage
```html
    convox racks
```
### Examples
```html
    $ convox racks
    NAME               PROVIDER  STATUS
    dev                local     running
    testing            do        running
    acme/production    aws       running
    acme/staging       gcp       running
    integration/test   azure     running
```