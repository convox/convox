---
title: "internal_router"
draft: false
slug: internal_router
url: /configuration/rack-parameters/aws/internal_router
---

# internal_router

## Description
The `internal_router` parameter installs an internal load balancer within the VPC. This load balancer is not exposed to the public internet and is used for internal traffic routing.

## Default Value
The default value for `internal_router` is `false`.

## Use Cases
- **Rack-to-Rack Communication**: Facilitates communication between different racks via VPC peering and routing.
- **Internal AWS Services**: Enables routing and data flows to other AWS services that are within the same VPC or connected through AWS backbone network.

## Setting Parameters
To enable the internal router, use the following command:
```html
$ convox rack params set internal_router=true -r rackName
Setting parameters... OK
```
This command installs an internal load balancer within the VPC.

## Additional Information
Using an internal load balancer can improve security by keeping internal traffic within the private network. This setup is beneficial for applications that need to communicate across different racks or utilize other AWS services internally.

By setting `internal_router` to `true`, you can leverage the benefits of internal load balancing, such as reduced latency and enhanced security for internal communications.

