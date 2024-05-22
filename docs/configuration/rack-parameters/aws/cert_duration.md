---
title: "cert_duration"
draft: false
slug: cert_duration
url: /configuration/rack-parameters/aws/cert_duration
---

# cert_duration

## Description
The `cert_duration` parameter specifies the renewal period for SSL/TLS certificates. This duration determines how frequently the certificates are renewed to ensure that your applications remain secure.

## Default Value
The default value for `cert_duration` is `2160h` (90 days).

## Use Cases
- **Custom Renewal Periods**: Adjusting the certificate renewal period to align with your security policies or operational requirements.
- **Compliance**: Ensuring that your SSL/TLS certificates are renewed within a specific timeframe to meet regulatory or compliance standards.

## Managing Parameters

### Viewing Current Parameters
```html
$ convox rack params -r rackName
cert_duration  2160h
node_disk  20
node_type  t3.small
```

### Setting Parameters
To set the `cert_duration` parameter, use the following command:
```html
$ convox rack params set cert_duration=4320h -r rackName
Setting parameters... OK
```
This command sets the certificate renewal period to 180 days.

## Additional Information
The renewal period for SSL/TLS certificates is crucial for maintaining the security of your applications. Shorter renewal periods can enhance security by ensuring that certificates are updated more frequently. However, this might increase the administrative overhead. Balance the renewal period with your security and operational needs to determine the optimal `cert_duration` value.
