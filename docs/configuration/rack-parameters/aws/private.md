---
title: "private"
draft: false
slug: private
url: /configuration/rack-parameters/aws/private
---

# private

## Description
The `private` parameter specifies whether to place nodes in private subnets behind NAT gateways. This is a security best practice as it limits direct exposure to the internet, protecting the nodes from external threats.

**Note**: The `private` parameter is immutable and cannot be changed once a rack has been created. Ensure you set this parameter correctly during the initial rack setup based on your security and network requirements.

## Default Value
The default value for `private` is `true`.

## Use Cases
- **Enhanced Security**: By placing nodes in private subnets, you reduce the attack surface area as the nodes are not directly accessible from the internet.
- **Compliance Requirements**: Certain compliance standards may require that sensitive workloads run in private subnets.

## Managing Parameters

### Viewing Current Parameters
```html
$ convox rack params -r rackName
private  true
```

### Setting Parameters
To set the `private` parameter, use the following command:
```html
$ convox rack params set private=false -r rackName
Setting parameters... OK
```
This command sets the nodes to be placed in public subnets.

## Additional Information
When the `private` parameter is set to `true`, nodes are placed in private subnets, which enhances security by preventing direct access from the internet.

Proper configuration of private subnets is essential to ensure network connectivity and security for your applications. By setting the `private` parameter, you can improve the security posture of your Convox rack.

