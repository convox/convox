---
title: "App Parameters"
description: "App parameters are per-application settings that control build and deployment behavior for a single app, set with convox apps params."
slug: app-parameters
url: /configuration/app-parameters
---
# App Parameters

App parameters are per-application configuration settings that control build and deployment behavior for a specific app. Unlike [Rack Parameters](/configuration/rack-parameters), which apply to the entire cluster (node types, networking, storage drivers), app parameters let you customize individual applications, such as directing builds to specific node groups or adjusting build resource limits.

## Managing App Parameters

### Viewing Current Parameters
To view the current app parameters, use the following command:
```bash
$ convox apps params -a appName
```
This command lists the app parameters that have a value stored for the App. Parameters that have never been set are not returned, so an App with no app parameters set produces no rows.

### Setting Parameters
To set an app parameter, use the following command:
```bash
$ convox apps params set parameterName=value -a appName
Updating parameters... OK
```
This command sets the specified parameter to the given value.

## Cloud Providers

- [Amazon Web Services (AWS)](/configuration/app-parameters/aws)

App parameters are implemented in provider-agnostic Rack code, so the same set applies on Racks for every provider, not only AWS. Use the AWS page above as the reference for all of them. Some of the Rack parameters they interact with, such as [`build_node_enabled`](/configuration/rack-parameters/aws/build_node_enabled), are AWS-specific.
