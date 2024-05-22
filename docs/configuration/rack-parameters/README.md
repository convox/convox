---
title: "Rack Parameters"
draft: false
slug: rack-parameters
url: /configuration/rack-parameters
---
# Rack Parameters

Rack parameters are configuration settings that control various aspects of your Convox rack. These parameters allow you to customize and optimize the behavior of your applications and services running on the rack.

## Managing Rack Parameters

### Viewing Current Parameters
To view the current rack parameters, use the following command:
```html
$ convox rack params -r rackName
```
This command displays the current values of all rack parameters for the specified rack.

### Setting Parameters
To set a rack parameter, use the following command:
```html
$ convox rack params set parameterName=value -r rackName
Setting parameters... OK
```
This command sets the specified parameter to the given value.

## Cloud Providers

- [Amazon Web Services (AWS)](/configuration/rack-parameters/aws)
- [Google Cloud Platform (GCP)](/configuration/rack-parameters/gcp)
- [Microsoft Azure](/configuration/rack-parameters/azure)
- [Digital Ocean](/configuration/rack-parameters/do)

Select your cloud provider to view the available parameters and their configurations.
