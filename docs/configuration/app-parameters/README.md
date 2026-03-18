---
title: "App Parameters"
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
This command displays the current values of all app parameters for the specified application.

### Setting Parameters
To set an app parameter, use the following command:
```bash
$ convox apps params set parameterName=value -a appName
Setting parameters... OK
```
This command sets the specified parameter to the given value.

## Cloud Providers

- [Amazon Web Services (AWS)](/configuration/app-parameters/aws)

Select your cloud provider to view the available parameters and their configurations.
