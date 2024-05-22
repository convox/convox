---
title: "build_node_enabled"
draft: false
slug: build_node_enabled
url: /configuration/rack-parameters/aws/build_node_enabled
---

# build_node_enabled

## Description
The `build_node_enabled` parameter enables a dedicated build node for building applications. This setup can help isolate build processes from application runtime environments, ensuring that builds do not impact the performance of running applications.

## Default Value
The default value for `build_node_enabled` is `false`.

## Use Cases
- **Isolated Build Environment**: Using a dedicated build node ensures that the resources used during the build process do not affect the application runtime environment.
- **Optimized Build Performance**: A dedicated build node can be optimized for build tasks, potentially speeding up the build process.

## Managing Parameters

### Viewing Current Parameters
```html
$ convox rack params -r rackName
build_node_enabled  false
node_disk  20
node_type  t3.small
```

### Setting Parameters
To enable a dedicated build node, use the following command:
```html
$ convox rack params set build_node_enabled=true -r rackName
Setting parameters... OK
```
This command enables the dedicated build node for your rack.

## Additional Information
Enabling a dedicated build node can improve the reliability and performance of your builds, especially for large or complex applications. This setup is particularly useful in CI/CD pipelines where frequent builds are required. Ensure that the build node type and configuration are suitable for your build requirements.
