--- 
title: "Rack-to-Rack Communication" 
draft: false 
slug: Rack-to-Rack Communication 
url: /configuration/rack-to-rack 
---
# Rack-to-Rack Communication 

Using Convox’s (internal_router)[/installation/production-rack/aws/] rack parameter along with service configuration (internalRouter)[/reference/primitives/app/service/] you can enable private communication between racks/clusters on your cloud platform.  You can complete this in a few simple steps: 

## Prerequisites
It is required that you are on rack version `3.10.6` or later.  You can check your rack's version by running `convox rack -r rackNAME`

You will first need to establish connectivity between your racks in your given cloud environment.  Under standard conditions rack's will be installed in seperate VPCs. Connectivity is most easily accomplished via VPC Peering and configuration of routes, and security groups.   

You will need to manually complete this setup process on your own as we cannot predict your existing infrastructure or how you would need to facilitate or secure this connection/peering to suit your requirements. 

## Configuration
Once connectivity is established you will need to change the (internalRouter)[/reference/primitives/app/service/] rack parameter to `true` by running: 
`convox rack params set internal_router=true –r rackNAME`
* This will install the internal loadbalancer into the VPC that facilitates rack-to-rack communication. 
* You can verify that the load balancer was created in your cloud environment by checking it’s applicable service page. 


Finally set your desired service to use this internal load balancer by configuring the service attribute (internalRouter)[/reference/primitives/app/service/] to `true` and deploy the application. 

```html
services: 
  web: 
    build: . 
    port: 3000 
    internalRouter: true 
    environment: 
      - PORT=3000 
```
* You can verify that this service is being internally routed by running `convox services –a appNAME` and attempting to access the service URL from the public internet and again from a service within your VPC peered Rack. 
