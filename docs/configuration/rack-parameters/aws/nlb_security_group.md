---
title: "nlb_security_group"
description: "The nlb_security_group AWS rack parameter attaches an existing security group to the rack's internet-facing router load balancer, defaulting to a group the rack creates and manages."
slug: nlb_security_group
url: /configuration/rack-parameters/aws/nlb_security_group
---

# nlb_security_group

## Description
The `nlb_security_group` parameter attaches an existing security group to the load balancer in front of the rack's internet-facing router. Use caution when setting it, because an improper security group can cut off access to every service on the rack.

This parameter applies to the rack's primary internet-facing router only. It does not apply to the internal load balancer installed by [internal_router](/configuration/rack-parameters/aws/internal_router), or to load balancers created by an app's `balancers` block.

## Default Value
The default value for `nlb_security_group` is an empty string, and it can be cleared back to empty at any time. When it is empty, the rack creates and manages a security group for the router load balancer. That managed group allows inbound traffic on the router's listener ports from the CIDRs in the `whitelist` rack parameter, which defaults to `0.0.0.0/0`.

## Version Requirements
This parameter takes effect only on load balancers created by the AWS Load Balancer Controller, which the rack has used since `3.18.0`. AWS does not allow a security group to be attached to a network load balancer that was created without one, so a rack whose router load balancer was created before `3.18.0` cannot use this parameter, and setting it has no effect.

Upgrading an older rack does not replace its load balancer, so the rack keeps the one it was installed with. Moving such a rack onto a load balancer that accepts a security group means replacing the load balancer, which changes its DNS name and requires a coordinated cutover. Contact Convox support to plan that change.

Racks in that state report the condition on every rack update:

```
WARNING: nlb_security_group is set but is not being applied to this rack's primary router load balancer.
```

Clear the parameter to remove it, so it no longer reports a value that is not in force:

```bash
$ convox rack params set nlb_security_group= -r rackName
Updating parameters... OK
```

To restrict inbound access on a rack in that state, use the `whitelist` parameter instead. On these racks `whitelist` is enforced as ingress rules on the worker node security groups, so it restricts which source addresses can reach the router from the internet. It does not restrict traffic originating inside the rack's own VPC subnets.

## Use Cases
- **Organization security policy**: Attach a security group managed by your security team so the rack load balancer sits inside your existing rule set.
- **Narrow ingress**: Restrict the source addresses and protocols that can reach the rack router.

## Setting Parameters
To set the `nlb_security_group` parameter, use the following command:
```bash
$ convox rack params set nlb_security_group=sg-12345678 -r rackName
Updating parameters... OK
```
This command attaches the specified security group to the router load balancer.

## Additional Information
On a rack whose router load balancer accepts a security group, setting your own replaces the group the rack manages. The `whitelist` parameter is enforced through that managed group, so once `nlb_security_group` is set, `whitelist` no longer restricts traffic at the load balancer. Your security group becomes the only inbound control at that point, and it must allow TCP 80 and TCP 443 from your intended clients or the router stops accepting traffic.

On a rack that cannot use this parameter, `whitelist` continues to apply as described under Version Requirements.

Carefully configure the security group to ensure that only trusted IP addresses and protocols can access your services. Improper settings may result in loss of access or expose your services to potential threats.

## Related Parameters
- [internal_router](/configuration/rack-parameters/aws/internal_router): Installs a separate internal load balancer. `nlb_security_group` does not apply to it.
- [proxy_protocol](/configuration/rack-parameters/aws/proxy_protocol): Also changes the router load balancer configuration.
