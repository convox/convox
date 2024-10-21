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

