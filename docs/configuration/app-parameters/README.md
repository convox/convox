---
title: "App Parameters"
draft: false
slug: app-parameters
url: /reference/app-parameters
---
# App Parameters

App parameters are configuration settings that control various aspects of your Convox applications. These parameters allow you to customize and optimize the behavior of your applications without modifying your application code.

## Managing App Parameters

### Viewing Current Parameters
To view the current app parameters, use the following command:
```html
$ convox apps params -a appName
```
This command displays the current values of all app parameters for the specified application.

### Setting Parameters
To set an app parameter, use the following command:
```html
$ convox apps params set parameterName=value -a appName
Setting parameters... OK
```
This command sets the specified parameter to the given value.

## Cloud Providers

- [Amazon Web Services (AWS)](/reference/app-parameters/aws)

Select your cloud provider to view the available parameters and their configurations.
