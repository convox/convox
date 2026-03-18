---
title: "App Definition"
slug: app-definition
url: /configuration/app-definition
---
# App Definition

Your application's runtime behavior is defined through a combination of configuration sources. Each source serves a different purpose.

## Environment Variables

Environment variables inject runtime configuration into your services. They can be defined at the app level (shared across all services) or at the service level, and managed through the CLI or Console.

See [Environment Variables](/configuration/environment) for details.

## Config Mounts

Config mounts let you inject configuration files into your containers as Kubernetes Secrets. Define file contents in the `configs` section of your `convox.yml` and mount them into services using `configMounts`.

See [Config Mounts](/configuration/config-mounts) for details.

## Volumes

Persistent storage is available through AWS EFS volumes, and ephemeral scratch space through emptyDir volumes. Volumes are mounted into services using the `volumes` section of your service definition.

See [Volumes](/configuration/volumes) for details.

## App Settings

App-level settings control platform behavior for your application, such as CloudWatch log retention.

See [App Settings](/configuration/app-settings) for details.

## Private Registries

If your application pulls base images from private Docker registries, you can configure authentication so builds can access them.

See [Private Registries](/configuration/private-registries) for details.
