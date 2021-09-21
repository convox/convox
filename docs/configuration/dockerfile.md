---
title: "Dockerfile"
draft: false
slug: Dockerfile
url: /configuration/dockerfile
---

# Dockerfile

The `Dockerfile` describes the steps used to create a [Build](/reference/primitives/app/build) from your
application code.
```html
    FROM ubuntu:18.04
    COPY . .
    RUN ["deps", "install"]
    CMD ["bin/start"]
```
## Common Directives

| Directive | Description                                      |
| --------- | ------------------------------------------------ |
| **FROM**    | defines the base image                           |
| **COPY**    | add files from the local directory tree          |
| **RUN**     | execute a command                                |
| **CMD**     | defines the default command to run on this image |
| **ARG**     | define build variables                           |

## Optimizing Build Times

Each line of a `Dockerfile` will be cached as long as files referenced by it are not changed. This allows you
to cache expensive steps such as dependency installation by selectively copying files before running commands.

The following example selectively copies only the files needed to run `npm` before installing dependencies.
```html
    FROM nodejs

    COPY package.json package-lock.json .
    RUN ["npm", "install"]

    COPY . .
    CMD ["npm", "start"]
```
The `npm install` will be cached on successive builds unless `package.json` or `package-lock.json` is changed.

## Build Variables

Convox respects the `ARG` directive, allowing you to specify variables at build time.

This is useful for creating dynamic build environments, allowing you to use the same Dockerfile for varying
deployments.

> It is not recommended to use build variables for passing secrets. Values for build variables are embedded
> in the resulting image.

You can declare build variables using the `ARG` directive with an optional default value:
```html
    ARG COPYRIGHT=2020
    ARG RUBY_VERSION
```
Values for these variables will be read from the [Environment](/configuration/environment) at build time:
```html
    $ convox env set RUBY_VERSION=2.6.4
```
## See Also

- [Dockerfile: Best Practices](https://docs.docker.com/develop/develop-images/dockerfile_best-practices/)
- [Private Registries](/configuration/private-registries)