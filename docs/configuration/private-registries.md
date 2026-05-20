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

## Build-Time vs. Runtime Registry Authentication

`convox registries` provides credentials used during `convox build` and `convox deploy` to pull base images referenced in your Dockerfile. These credentials are consumed by the build process and do not affect running containers.

To authenticate at **runtime** — when Kubernetes pulls the container image specified in the `image` field of `convox.yml` — use the [`imagePullSecrets`](/reference/primitives/app/service#imagepullsecrets) field on a Service:

```yaml
services:
  nim:
    image: nvcr.io/nim/meta/llama-3.1-8b-instruct:latest
    imagePullSecrets:
      - registry: nvcr.io
        username: $oauthtoken
        passwordEnv: NGC_API_KEY
```

| Scenario | Mechanism |
|----------|-----------|
| Pulling base images during `convox build` | `convox registries add` |
| Pulling a pre-built `image` at deploy/runtime | `imagePullSecrets` in `convox.yml` |

Both can be used in the same App if a Service builds from a private base image and also runs a separate Service from a pre-built private image.

See [Service imagePullSecrets](/reference/primitives/app/service#imagepullsecrets) for field reference and validation rules.

## See Also

- [docker_hub_username](/configuration/rack-parameters/aws/docker_hub_username) and [docker_hub_password](/configuration/rack-parameters/aws/docker_hub_password) for authenticating Docker Hub pulls across all Convox-managed pods
- [ecr_docker_hub_cache](/configuration/rack-parameters/aws/ecr_docker_hub_cache) for an AWS ECR pull-through cache of Docker Hub images on resource pods
