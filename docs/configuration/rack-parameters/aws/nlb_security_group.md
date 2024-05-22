---
title: "nlb_security_group"
draft: false
slug: nlb_security_group
url: /configuration/rack-parameters/aws/nlb_security_group
---

# nlb_security_group

## Description
The `nlb_security_group` parameter specifies the ID of the security group to attach to the Network Load Balancer (NLB). Use caution when configuring this parameter to avoid losing access to services due to improper security group settings.

## Default Value
The default value for `nlb_security_group` is an empty string. When set to an empty string, Convox will apply the AWS default NLB security group, which allows inbound traffic from any IP address.

## Use Cases
- **Custom Security Group**: Attach a custom security group to the NLB to control inbound and outbound traffic according to your security policies.
- **Enhanced Security**: Restrict access to the NLB by specifying a security group that defines allowed IP ranges and protocols.

## Setting Parameters
To set the `nlb_security_group` parameter, use the following command:
```html
$ convox rack params set nlb_security_group=sg-12345678 -r rackName
Setting parameters... OK
```
This command attaches the specified security group to the NLB.

## Additional Information
When the `nlb_security_group` parameter is set to ``, Convox will apply the AWS default security group to the NLB, which allows inbound traffic from any IP address. For enhanced security, it is recommended to specify a custom security group that restricts access according to your requirements.

Carefully configure the security group to ensure that only trusted IP addresses and protocols can access your services. Improper settings may result in loss of access or expose your services to potential threats.

By setting the `nlb_security_group` parameter, you can ensure that your NLB operates within the security framework defined by your organization, providing controlled and secure access to your applications.
