---
title: "cert_duration"
draft: false
slug: cert_duration
url: /configuration/rack-parameters/gcp/cert_duration
---

# cert_duration

## Description
The `cert_duration` parameter specifies the certification renewal period. This determines how often your certificates are renewed, ensuring they remain valid and secure.

## Default Value
The default value for `cert_duration` is `2160h` (90 days).

## Use Cases
- **Security Maintenance**: Regularly renewing certificates helps maintain secure communication channels.
- **Compliance**: Ensuring certificates are renewed within a specific period can help meet compliance requirements.

## Setting Parameters
To set the `cert_duration` parameter, use the following command:
```html
$ convox rack params set cert_duration=2160h -r rackName
Setting parameters... OK
```
This command sets the `cert_duration` parameter to the specified value.

## Additional Information
Adjusting the `cert_duration` can help balance between operational convenience and security requirements. Shorter renewal periods increase security but require more frequent updates, while longer periods reduce the maintenance overhead.
