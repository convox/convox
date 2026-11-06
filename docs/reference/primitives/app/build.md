---
title: "Build"
description: "A Build is a compiled version of the code for each Service of an App, produced from a Dockerfile, with each Build creating a new Release."
slug: build
url: /reference/primitives/app/build
---
# Build

A Build is a compiled version of the code for each [Service](/reference/primitives/app/service) of an [App](/reference/primitives/app).

The Rack builds the image for each Service from its Dockerfile with BuildKit. `convox build --external` builds it with Docker on your own machine instead; see [External Builds](/reference/cli/build#external-builds).

## Build Definition

You can define the location to build for each [Service](/reference/primitives/app/service) in [`convox.yml`](/configuration/convox-yml).

```yaml
services:
  api:
    build: ./api
  web:
    build:
      path: ./web
      manifest: Dockerfile.production
```

## Command Line Interface

### Creating a Build

```bash
    $ convox build -a myapp
    Packaging source... OK
    Uploading source... OK
    Starting build... OK
    ...
    Build: BABCDEFGHI
    Release: RBCDEFGHIJ
```

> Every time a Build is created a new [Release](/reference/primitives/app/release) is created that references the Build.

### Listing Builds

```bash
    $ convox builds -a myapp
    ID          STATUS    RELEASE     STARTED       ELAPSED
    BABCDEFGHI  complete  RBCDEFGHIJ  1 minute ago  25s
```

### Getting Information about a Build

```bash
    $ convox builds info BABCDEFGHI -a myapp
    Id           BABCDEFGHI
    Status       complete
    Release      RBCDEFGHIJ
    Description
    Started      1 minute ago
    Elapsed      25s
```

A Build that failed or was cancelled also prints a `Reason` row.

### Getting logs for a Build

```bash
    $ convox builds logs BABCDEFGHI -a myapp
    #1 [internal] load build definition from Dockerfile
    #1 transferring dockerfile: 133B done
    #1 DONE 0.2s

    #2 [internal] load metadata for docker.io/library/alpine:3.20
    #2 DONE 0.5s
    ...
```

### Cancelling a Build

```bash
    $ convox builds cancel BABCDEFGHI -a myapp
    Cancelling build BABCDEFGHI... OK
```

> Cancelling a Build requires rack version 3.25.7 or later.

Cancelling deletes the build pod and sets the Build to `failed`; `convox builds info` then carries a `Reason` of `cancelled by <actor>`. Only a Build the Rack is running in a build pod can be cancelled. A cancelled Build has no logs, because a Build uploads its logs when it finishes. See [builds](/reference/cli/builds#builds-cancel).

### Exporting a Build

```bash
    $ convox builds export BABCDEFGHI -a myapp -f /tmp/build.tgz
    Exporting build... OK
```

### Importing a Build

```bash
    convox builds import -a myapp2 -f /tmp/build.tgz
```

> Importing a Build creates a new [Release](/reference/primitives/app/release) that references the Build.

## Build Arguments

> Build arguments require rack version 3.22.0 or later.

Convox supports both custom build arguments and Convox-managed build arguments that provide build context information. Build arguments must be declared in your Dockerfile using the `ARG` instruction to be available during the build process.

### Convox-Managed Build Arguments

Starting from version 3.22.0, Convox provides the following managed build arguments:

| Argument | Description |
|----------|-------------|
| `BUILD_APP` | The application name |
| `BUILD_AUTH` | Docker registry authentication credentials (contains sensitive data) |
| `BUILD_DEVELOPMENT` | Development environment flag |
| `BUILD_GENERATION` | Build generation identifier |
| `BUILD_ID` | Unique build identifier |
| `BUILD_MANIFEST` | Build manifest information |
| `BUILD_RACK` | Rack name |
| `BUILD_GIT_SHA` | Git commit SHA for the build |

### Using Build Arguments via CLI

Pass Convox-managed build arguments:
```bash
    $ convox build --build-args "BUILD_APP" --build-args "BUILD_ID" --build-args "BUILD_GIT_SHA"
```

Combine Convox-managed arguments with custom build arguments:
```bash
    $ convox build --build-args "BUILD_APP" --build-args "FOO=BAR" --build-args "VERSION=1.2.3"
```

### Dockerfile Configuration

**Important**: Build arguments must be explicitly declared in your Dockerfile using the `ARG` instruction to be available during the build:

```dockerfile
# Declare Convox-managed args
ARG BUILD_APP
ARG BUILD_ID
ARG BUILD_GIT_SHA
ARG BUILD_RACK

# Declare custom args
ARG FOO
ARG VERSION

# Use the args in your build
RUN echo "Building app: ${BUILD_APP} with version: ${BUILD_GIT_SHA}"

# Pass args to runtime environment if needed
ENV APP_VERSION=${BUILD_GIT_SHA}
ENV APP_NAME=${BUILD_APP}
```

### Security Warning for BUILD_AUTH

**Critical Security Notice**: The `BUILD_AUTH` argument contains sensitive Docker registry authentication credentials including usernames and passwords/tokens. This data should never be:
- Logged or printed during builds
- Embedded in final images
- Exposed in build output

Example of `BUILD_AUTH` content structure (contains sensitive data):
```text
BUILD_AUTH: {"registry.url":{"Username":"AWS","Password":"[base64-encoded-token]"}}
```

Only use `BUILD_AUTH` when necessary for authenticating to private registries, and ensure it is not persisted in your final image layers.

### Using Build Arguments with CI/CD Workflows

When using Convox Deployment and Review Workflows:
1. Navigate to your workflow configuration modal
2. Check the "Make Convox Managed Build Args Available" checkbox to include all Convox-managed build arguments
3. Use the custom build args input field to add your own arguments (e.g., `FOO=BAR,VERSION=1.2.3`)

## Build Layers Caching

A Build imports a cache of each Service's layers when it starts and exports the cache again when it finishes. Dockerfile steps whose inputs have not changed print `CACHED` and do not run. Within one Build, Services built from the same Dockerfile reuse each other's layers on every provider. If the registry rejects the cache export, the Build still succeeds. To build without the cache, see [Building Without the Cache](/reference/cli/build#building-without-the-cache).

| Provider | Where the cache is kept |
|----------|-------------------------|
| AWS | The App's ECR repository, tag `<service>.buildcache`. On by default; setting [`disable_image_manifest_cache`](/configuration/rack-parameters/aws/disable_image_manifest_cache) to `true` turns it off. With [`ecr_immutable_tags_enabled`](/configuration/rack-parameters/aws/ecr_immutable_tags_enabled) on Rack version 3.25.10 or later, each Build writes `<service>.buildcache.<build id>`. [`buildkit_host_path_cache_enable`](/configuration/rack-parameters/aws/buildkit_host_path_cache_enable) adds a cache on the build node |
| GCP | The App's Artifact Registry repository, tag `<service>.buildcache` |
| DigitalOcean | The App's repository in the registry the Rack runs, tag `<service>.buildcache` |
| Azure | The App's repository in the Rack's Azure Container Registry, tag `<service>.buildcache`. Requires Rack version 3.25.12 or later; earlier Racks keep no cache between Builds |
| Local and metal | Inside the build Process only; nothing is kept after the Build |

The first Build of each Service, and every Build of a Service with no build steps of its own such as an `image:` Service, prints `ERROR: failed to configure registry cache importer: <repository>:<service>.buildcache: not found` and builds without the cache; the Build continues. This happens wherever the cache is kept in the registry, except on AWS with `ecr_immutable_tags_enabled` on.

### Cache Storage on Azure

Each Build moves the `<service>.buildcache` tag to a new cache manifest and leaves the previous one untagged. Convox deletes nothing from an Azure registry, so untagged cache manifests and Build images stay until you remove them. The Rack's registry is the Basic tier, which includes 10 GiB of storage and bills storage above that per GiB.

The ACR retention policy for untagged manifests does not cover these: it requires the Premium tier and skips OCI manifests, and the cache manifest is OCI. To remove untagged manifests, run `acr purge --untagged-only` as an ACR Task. `acr purge` is a preview feature of Azure Container Registry. Run it with `--dry-run` first to list what it would delete:

```bash
$ az acr run --registry <registry> --cmd "acr purge --untagged-only --ago 7d --dry-run" /dev/null
```

Remove `--dry-run` to delete. To run it on a schedule, create a task:

```bash
$ az acr task create --name purge-untagged --cmd "acr purge --untagged-only --ago 7d" --schedule "0 0 * * *" --registry <registry> --context /dev/null
```

`<registry>` is the name of the Rack's registry, which `az acr list -o table` shows. Use `--untagged-only`, not `--filter` with `--ago`: a filter deletes tags older than `--ago`, including the Build images that Releases run from.

## Using Docker Credentials in Builds

### Docker Hub Rate Limiting

Convox supports using Docker Hub credentials to avoid rate limits imposed on anonymous pulls (100 pulls per 6 hours per source IP). When `docker_hub_username` and `docker_hub_password` are set as rack parameters, Docker Hub credentials are attached to build pods, `convox run` pods, service deployments, resource deployments (Redis, Postgres, MySQL, MariaDB, Memcached, PostGIS), and timer CronJobs, so every Convox-managed pull from Docker Hub is authenticated.

For AWS racks, [`ecr_docker_hub_cache`](/configuration/rack-parameters/aws/ecr_docker_hub_cache) complements Docker Hub credentials by serving resource images through an ECR pull-through cache, eliminating per-node Docker Hub pulls during node churn.

### How to Use Docker Credentials in Convox

To use Docker Hub credentials during the build process, follow these steps:

1. **Generate a read-only access token in Docker Hub:**
   - Log in to your Docker Hub account.
   - Go to **Account Settings** and navigate to the **Security** tab.
   - Under **Access Tokens**, click **New Access Token**.
   - Set the access permissions to **Read-only** and generate the token.

2. **Set the credentials for your Convox rack:**

   Run the following command to set the Docker Hub credentials on your rack. Be sure to use the read-only access token to avoid storing your Docker password in plain text.

   ```bash
   $ convox rack params set docker_hub_username=<your-docker-hub-username> docker_hub_password=<your-read-only-token> -r <rackName>
   ```

3. **Verify that the credentials have been set:**

   After setting the credentials, you can confirm they have been successfully configured by running:

   ```bash
   $ convox rack params -r <rackName>
   ```

This will list the current parameters for the rack, including the Docker credentials.

## Version Requirements

- Basic build functionality: All versions
- Convox-managed build arguments: Version 3.22.0+
- `convox builds cancel`: Version 3.25.7+
- Registry Build cache on Azure: Rack version 3.25.12 or later
