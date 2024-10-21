---
title: "syslog"
draft: false
slug: syslog
url: /configuration/rack-parameters/do/syslog
---

# syslog

## Description
The `syslog` parameter specifies the endpoint to forward logs to a syslog server (e.g. **tcp+tls://example.org:1234**).

## Default Value
The default value for `syslog` is ``.

## Use Cases
- **Centralized Logging**: Forward logs to a central syslog server for easier monitoring and analysis.
- **Compliance**: Meet compliance requirements by ensuring all logs are collected and stored in a central location.

## Setting Parameters
To set the `syslog` parameter, use the following command:
```html
$ convox rack params set syslog=tcp+tls://example.org:1234 -r rackName
Setting parameters... OK
```
This command sets the `syslog` parameter to the specified value.

## Additional Information
When set to ``, syslog forwarding is not enabled. This parameter is optional and can be configured based on your specific logging needs. Ensure that the syslog endpoint is reachable and properly configured to receive logs from your Convox rack.
