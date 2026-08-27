---
title: "nlb_security_group"
description: "The nlb_security_group AWS rack parameter attaches an existing security group to the rack's internet-facing router load balancer, defaulting to a group the rack creates and manages."
slug: nlb_security_group
url: /configuration/rack-parameters/aws/nlb_security_group
---

# nlb_security_group

## Description
The `nlb_security_group` parameter attaches an existing security group to the router load balancer, the NLB in front of the Rack's internet-facing router. Use caution when setting it, because an improper security group can cut off access to every Service on the Rack.

It does not apply to the internal load balancer installed by [internal_router](/configuration/rack-parameters/aws/internal_router), or to load balancers created by an App's `balancers` block.

## Default Value
The default value for `nlb_security_group` is an empty string. When it is empty, the Rack creates and manages a security group for the router load balancer, which allows inbound traffic on the router's listener ports from the CIDRs in the `whitelist` Rack parameter.

## Use Cases
- **Organization security policy**: Attach a security group managed by your security team so the router load balancer sits inside your existing rule set.
- **Narrow ingress**: Restrict the source addresses and protocols that can reach the Rack router.

## Setting Parameters
To set the `nlb_security_group` parameter, use the following command:
```bash
$ convox rack params set nlb_security_group=sg-12345678 -r rackName
Updating parameters... OK
```
This command attaches the specified security group to the router load balancer.

## Additional Information
Setting your own security group replaces the group the Rack manages. Because `whitelist` is enforced through that managed group, it no longer restricts traffic at the router load balancer once `nlb_security_group` is set. Your security group is then the only inbound control at the load balancer, and it must allow TCP 80 and TCP 443 from your intended clients or the router stops accepting traffic.

## Rack Update Messages
Every Rack update checks whether `nlb_security_group` reaches the router load balancer and prints the result to stderr. The check runs in the `convox` binary performing the apply, which for a Console-managed Rack is the CLI bundled in the Console deploy rather than the CLI on your machine. See [CLI Rack Management](/management/cli-rack-management).

### The parameter is set but not applied

```text
WARNING: nlb_security_group is set but is not being applied to this rack's primary router load balancer. That load balancer is not managed by the AWS Load Balancer Controller, so the security group has no effect on it. Attaching one requires replacing the load balancer, so contact Convox support to plan that change.
```

The router load balancer on a Rack in this state predates `3.18.0` and cannot take a security group, as described under Version Requirements. Replacing it with a load balancer that can changes its DNS name and requires a coordinated cutover, so contact Convox support to plan that change.

Clear the parameter so the Rack no longer stores a value that is not in force:

```bash
$ convox rack params set nlb_security_group= -r rackName
Updating parameters... OK
```

Clearing is accepted at any time and returns the router load balancer to the Rack-managed security group.

### The check could not run

```text
NOTICE: could not check whether nlb_security_group applies to this rack: <error>
```

This prints when the Rack's router Service cannot be read. On a Rack reached through a private endpoint host the error detail is omitted, because a transport error carries the request URL:

```text
NOTICE: could not check whether nlb_security_group applies to this rack
```

Neither message fails the apply, and neither reports whether the parameter is applied.

## Related Parameters
- [internal_router](/configuration/rack-parameters/aws/internal_router): Installs a separate internal load balancer. `nlb_security_group` does not apply to it.
- [proxy_protocol](/configuration/rack-parameters/aws/proxy_protocol): Also changes the router load balancer configuration.

## Version Requirements
This parameter takes effect only on load balancers created by the AWS Load Balancer Controller, which the Rack has used since `3.18.0`. AWS does not allow a security group to be attached to an NLB that was created without one, so a Rack whose router load balancer was created before `3.18.0` cannot use this parameter, and setting it has no effect.

Upgrading an older Rack does not replace its load balancer, so the Rack keeps the one it was installed with.
