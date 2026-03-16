---
title: "Private Registries"
slug: private-registries
url: /configuration/private-registries
---
# Private Registries

Convox can pull base images from private registries during the build process.

## When to Use Private Registries

Registered credentials are used during `convox build` and `convox deploy` to authenticate with private Docker registries when pulling base images specified in your Dockerfile. This enables your builds to access images from registries that require authentication, such as Docker Hub (for rate limit avoidance), Amazon ECR, GitHub Container Registry, or any other private registry.

## Managing Registry Credentials

### Adding a Registry
```bash
    $ convox registries add registry.example.org username password
    Adding registry... OK
```
### Listing Registries
```bash
    $ convox registries
    SERVER                USERNAME
    registry.example.org  username
```
### Removing a Registry
```bash
    $ convox registries remove registry.example.org
    Removing registry... OK
```