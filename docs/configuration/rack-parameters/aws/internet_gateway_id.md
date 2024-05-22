---
title: "internet_gateway_id"
draft: false
slug: internet_gateway_id
url: /configuration/rack-parameters/aws/internet_gateway_id
---

# internet_gateway_id

## Description
The `internet_gateway_id` parameter is used when you are using an existing VPC for your rack. This parameter allows you to specify the ID of the attached internet gateway.

## Default Value
The default value for `internet_gateway_id` is ``. When the `internet_gateway_id` parameter is set to ``, Convox will automatically create an internet gateway if one does not already exist for the VPC.

## Use Cases
- **Existing VPC Integration**: Use this parameter to integrate your Convox rack with an existing VPC that has an internet gateway attached.
- **Network Configuration**: Ensures that your rack can access the internet through the specified internet gateway.

## Setting Parameters
To set the `internet_gateway_id` parameter, use the following command:
```html
$ convox rack params set internet_gateway_id=igw-1234567890abcdef0 -r rackName
Setting parameters... OK
```
This command specifies the ID of the internet gateway for your existing VPC.

## Additional Information
By setting the `internet_gateway_id` parameter, you enable your rack to utilize the specified internet gateway, ensuring seamless integration with your existing AWS network infrastructure.

It is also important to configure the [cidr](/configuration/rack-parameters/aws/cidr) block to avoid collisions with existing VPC subnets. To avoid CIDR block collision with existing VPC subnets, please add a new CIDR block to your VPC to separate rack resources.
