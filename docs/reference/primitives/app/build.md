---
title: "Build"
draft: false
slug: Build
url: /reference/primitives/app/build
---
# Build

A Build is a compiled version of the code for each [Service](/reference/primitives/app/service) of an [App](/reference/primitives/app).

Convox uses `docker` to compile code into a Build.

## Definition

You can define the location to build for each [Service](/reference/primitives/app/service) in [`convox.yml`](/configuration/convox-yml).

```html
    services:
      api:
        build: ./api
      web:
        build: ./web
        manifest: Dockerfile.production
```

## Command Line Interface

### Creating a Build

```html
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

```html
    $ convox builds -a myapp
    ID          STATUS    RELEASE     STARTED       ELAPSED
    BABCDEFGHI  complete  RBCDEFGHIJ  1 minute ago  25s
```

### Getting Information about a Build

```html
    $ convox builds info BABCDEFGHI -a myapp
    ID           BABCDEFGHI
    Status       complete
    Release      RBCDEFGHIJ
    Description
    Started      1 minute ago
    Elapsed      25s
```

### Getting logs for a Build

```html
    $ convox builds logs BABCDEFGHI -a myapp
    Sending build context to Docker daemon  4.2MB
    Step 1/34 : FROM golang AS development
    ...
```

### Exporting a Build

```html
    $ convox builds export BABCDEFGHI -a myapp -f /tmp/build.tgz
    Exporting build... OK
```

### Importing a Build

```html
    convox builds import -a myapp2 -f /tmp/build.tgz
```

> Importing a Build creates a new [Release](/reference/primitives/app/release) that references the Build.

## Build Arguments

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
```html
    $ convox build --build-args=BUILD_APP --build-args=BUILD_ID --build-args=BUILD_GIT_SHA
```

Combine Convox-managed arguments with custom build arguments:
```html
    $ convox build --build-args=BUILD_APP --build-args=FOO=BAR --build-args=VERSION=1.2.3
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

⚠️ **Critical Security Notice**: The `BUILD_AUTH` argument contains sensitive Docker registry authentication credentials including usernames and passwords/tokens. This data should never be:
- Logged or printed during builds
- Embedded in final images
- Exposed in build output

Example of `BUILD_AUTH` content structure (contains sensitive data):
```
BUILD_AUTH: {"registry.url":{"Username":"AWS","Password":"[base64-encoded-token]"}}
```

Only use `BUILD_AUTH` when necessary for authenticating to private registries, and ensure it is not persisted in your final image layers.

### Using Build Arguments with CI/CD Workflows

When using Convox Deployment and Review Workflows:
1. Navigate to your workflow configuration modal
2. Check the "Make Convox Managed Build Args Available" checkbox to include all Convox-managed build arguments
3. Use the custom build args input field to add your own arguments (e.g., `FOO=BAR,VERSION=1.2.3`)

### Build Layers Caching

From version 3.11.0 onward, Convox uses buildkit to build and push images. Buildkit allows us to specify a caching path in remote repositories to store/fetch layers that have already been created. Unfortunately, the only rack registries that support such feature so far are Azure and DigitalOcean(DO racks have a built-in registry).

## Using Docker Credentials in Builds

### Overview

We have added support for using Docker credentials in Convox build and service pods. This feature helps avoid potential rate limits imposed by Docker Hub, particularly when operating large clusters that may perform multiple simultaneous pulls from Docker. By supplying Docker credentials, you can ensure that Docker Hub's rate limits are bypassed, resulting in smoother operations for your services.

### Requirements

- You must be on at least Convox rack version `3.18.8` to use this feature. 

### How to Use Docker Credentials in Convox

To use Docker Hub credentials during the build process, follow these steps:

1. **Generate a read-only access token in Docker Hub:**
   - Log in to your Docker Hub account.
   - Go to **Account Settings** and navigate to the **Security** tab.
   - Under **Access Tokens**, click **New Access Token**.
   - Set the access permissions to **Read-only** and generate the token.

2. **Set the credentials for your Convox rack:**

   Run the following command to set the Docker Hub credentials on your rack. Be sure to use the read-only access token to avoid storing your Docker password in plain text.

   ```html
   $ convox rack params set docker_hub_username=<your-docker-hub-username> docker_hub_password=<your-read-only-token> -r <rackName>
   ```

3. **Verify that the credentials have been set:**

   After setting the credentials, you can confirm they have been successfully configured by running:

   ```html
   $ convox rack params -r <rackName>
   ```

This will list the current parameters for the rack, including the Docker credentials.

## Version Requirements

- Basic build functionality: All versions
- Buildkit and caching: Version 3.11.0+
- Docker credentials support: Version 3.18.8+
- Convox-managed build arguments: Version 3.22.0+