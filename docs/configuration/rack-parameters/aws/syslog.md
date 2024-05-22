---
title: "syslog"
draft: false
slug: syslog
url: /configuration/rack-parameters/aws/syslog
---

# syslog

## Description
The `syslog` parameter specifies the endpoint to forward logs to a syslog server (e.g. **tcp+tls://example.org:1234**).

## Default Value
The default value for `syslog` is `null`. When set to `null`, syslog forwarding is not enabled. This parameter is optional and can be configured based on your specific logging needs.

## Use Cases
- **Centralized Logging**: Forward logs to a centralized syslog server for better log management and analysis.
- **Compliance and Auditing**: Ensure that logs are forwarded to a secure and centralized location to meet compliance and auditing requirements.

## Managing Parameters

### Viewing Current Parameters
```html
$ convox rack params -r rackName
syslog  
```

### Setting Parameters
To set the `syslog` parameter, use the following command:
```html
$ convox rack params set syslog='tcp+tls://example.org:1234' -r rackName
Setting parameters... OK
```
This command sets the syslog endpoint to the specified value.

## Additional Information
Configuring the `syslog` parameter allows you to forward logs to a specified syslog server, providing a centralized logging solution for better management and analysis. Ensure that the syslog server is correctly configured to receive logs from your Convox rack.