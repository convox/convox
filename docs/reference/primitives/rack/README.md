---
title: "Rack"
draft: false
slug: Rack
url: /reference/primitives/rack
---
# Rack

A Rack is a platform to build, deploy and host your [Apps](/reference/primitives/app).  It runs either locally on your own machine for development purposes or within your cloud infrastructure.

## Command Line Interface

### Getting information about a Rack
```html
    $ convox rack -r myrack
    Name      myrack
    Provider  aws
    Router    router.0a1b2c3d4e5f.convox.cloud
    Status    running
    Version   3.0.0

    $ convox rack params -r myrack
```
### Configuring a Rack
```html
    $ convox rack params set node_type=t3.medium
    Updating parameters... Upgrading modules...
    Downloading github.com/convox/convox?ref=3.0.0 for system...
    ...
```
### Updating a Rack
```html
    $ convox rack update -r myrack
    Upgrading modules...
    Downloading github.com/convox/convox?ref=3.0.0 for system...
    ...
    Apply complete! Resources: 0 added, 12 changed, 0 destroyed.

    Outputs:

    api = https://convox:password@api.0a1b2c3d4e5f.convox.cloud
    provider = aws
```
### Retrieving Rack logs
```html
    $ convox rack logs -r myrack
    ...
```
### Uninstalling a Rack
```html
    $ convox rack uninstall myrack
```