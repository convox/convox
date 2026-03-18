---
title: "Rack"
slug: rack
url: /reference/primitives/rack
---
# Rack

A Rack is a platform to build, deploy and host your [Apps](/reference/primitives/app). It runs either locally on your own machine for development purposes or within your cloud infrastructure.

## Primitives

| Primitive | Description |
|:----------|:------------|
| [Instance](/reference/primitives/rack/instance) | A compute node in the Rack's Kubernetes cluster. Instances provide CPU and memory capacity for running Processes. |
| [Registry](/reference/primitives/rack/registry) | The Rack's private container image registry. Stores Build images and serves them to the cluster during deployments. |

## Command Line Interface

### Getting information about a Rack
```bash
    $ convox rack -r myrack
    Name      myrack
    Provider  aws
    Router    router.0a1b2c3d4e5f.convox.cloud
    Status    running
    Version   3.23.3

    $ convox rack params -r myrack
```
### Configuring a Rack
```bash
    $ convox rack params set node_type=t3.medium
    Updating parameters... Upgrading modules...
    Downloading github.com/convox/convox?ref=3.23.3 for system...
    ...
```
### Updating a Rack
```bash
    $ convox rack update -r myrack
    Upgrading modules...
    Downloading github.com/convox/convox?ref=3.23.3 for system...
    ...
    Apply complete! Resources: 0 added, 12 changed, 0 destroyed.

    Outputs:

    api = https://convox:password@api.0a1b2c3d4e5f.convox.cloud
    provider = aws
```
### Retrieving Rack logs
```bash
    $ convox rack logs -r myrack
    ...
```
### Uninstalling a Rack
```bash
    $ convox rack uninstall myrack
```