---
title: "eks_control_plane_private_subnets"
description: "The eks_control_plane_private_subnets AWS rack parameter places the EKS control-plane network interfaces in the private subnets, defaulting to false."
slug: eks_control_plane_private_subnets
url: /configuration/rack-parameters/aws/eks_control_plane_private_subnets
---

# eks_control_plane_private_subnets

## Description

The `eks_control_plane_private_subnets` parameter places the network interfaces of the EKS control plane in the Rack's private subnets instead of its public subnets. EKS runs the Kubernetes API server outside the Rack's VPC and connects it to the VPC through 2 to 4 network interfaces it creates in the cluster's subnets. Each interface has the description `Amazon EKS <cluster-name>`.

With `false`, the interfaces are in the Rack's public subnets, where Convox has always created the cluster. With `true`, they are in the Rack's private subnets.

| Item | With `true` |
|------|-------------|
| EKS control-plane network interfaces | Move to the private subnets |
| Private IP addresses behind the API endpoint hostname | Change to addresses in the private subnets |
| Nodes, load balancers, NAT gateways, EFS mount targets | Unchanged |
| Endpoint access (public, public and private, or private only) | Unchanged |
| Security groups on the interfaces, including the rules from [eks_api_server_private_access_cidrs](/configuration/rack-parameters/aws/eks_api_server_private_access_cidrs) | Unchanged |
| EKS cluster | Updated in place, not replaced |

EKS gives these interfaces no public IP address in either placement. The parameter changes which subnets and route tables serve them, not who can reach the API server. If the cluster's subnets are changed by hand in the EKS console, the next Rack update sets them back to the subnets this parameter selects.

## Default Value

The default value is `false`. `convox rack params` lists the parameter only after it has been set, also under `-g network` and `-g security`; an unset parameter is `false`. The parameter cannot be cleared. Set it to `false` to go back.

## Before You Turn It On

| Your setup | Before turning it on |
|------------|----------------------|
| You reach the cluster only through `convox`, Convox Console, `convox rack kubeconfig`, the public API endpoint, or from inside the VPC | Nothing. `convox rack kubeconfig` points `kubectl` at the Rack API, not at the EKS endpoint, and traffic inside the VPC uses the `local` route |
| Rack installed with `private=false` in subnets Convox created | Not available. The Rack has no private subnets |
| Rack installed with `vpc_id` and no subnet lists | Convox created the subnets and route tables inside your VPC, so the rows below apply as for a new VPC |
| Rack installed into existing subnets with `private_subnets_ids` and `public_subnets_ids` | 2 or more private subnets, with at least one in every availability zone of `public_subnets_ids` and none in another zone, each with 6 or more free IP addresses (AWS recommends 16). Return routes go on your own route tables, see [Racks in Existing Subnets](#racks-in-existing-subnets) |
| Something outside the VPC uses the cluster's private API endpoint: an office VPN, a peered VPC, a transit gateway or Direct Connect | Add return routes to the private route tables first, see [Preparing Routes](#preparing-routes) |
| A firewall or allowlist names the API server's private IP addresses | Update it after the change, see [New Interface Addresses](#new-interface-addresses) |

## Why Routes Matter

A reply from the API server leaves the interface the request arrived on and follows the route table of the subnet that interface is in. In a VPC where Convox created the subnets, Convox creates these route tables:

| Route table `Name` tag | Count | Routes Convox creates |
|------------------------|-------|-----------------------|
| `<rack> public` | 1 | VPC CIDR to `local`; `0.0.0.0/0` to the internet gateway |
| `<rack> private 0`, `<rack> private 1`, `<rack> private 2` | 3 with `high_availability=true`, 2 otherwise | VPC CIDR to `local`; `0.0.0.0/0` to the NAT gateway in the same availability zone |

`<rack>` is the Rack name, which is also the EKS cluster name. On a Console Rack it is the Rack name without the organization prefix. Each route table also carries the tag `Rack=<rack>`, and each private subnet has the same `Name` as its route table.

If a route to a client network, for example `10.50.0.0/16` to `tgw-0123456789abcdef0`, exists only on the public route table, replies from interfaces in the private subnets follow `0.0.0.0/0` to the NAT gateway instead, and the client's connection times out.

Convox creates no other routes and does not change or remove routes it did not create, so a route you add to a private route table stays through every Rack update. On a Rack installed into existing subnets, Convox creates no route tables; the route tables associated with your private subnets apply.

On the far side of the connection, a network that routes the whole VPC CIDR to the Rack's VPC needs no change. A network that routes only the public subnet ranges also needs the private subnet ranges, listed under [New Interface Addresses](#new-interface-addresses).

Traffic inside the VPC, from nodes and Pods, uses the `local` route in every table and is not affected.

## Preparing Routes

Run these steps before turning the parameter on, with AWS CLI credentials for the Rack's account and region. The Route tables page of the VPC console does the same work.

1. Check the endpoint access and get the VPC id:

   ```bash
   $ aws eks describe-cluster --name my-rack --query 'cluster.resourcesVpcConfig.{private:endpointPrivateAccess,public:endpointPublicAccess,vpc:vpcId}'
   ```

   If `private` is `false`, nothing outside the VPC can use the private endpoint, so go to [Turning It On](#turning-it-on). `aws eks list-clusters` lists the cluster names in the region.

2. List the route tables in the VPC:

   ```bash
   $ aws ec2 describe-route-tables --filters Name=vpc-id,Values=vpc-0123456789abcdef0 --query 'RouteTables[].[Tags[?Key==`Name`]|[0].Value,RouteTableId]' --output table
   ```

   The output lists `my-rack public`, `my-rack private 0`, `my-rack private 1` and, with `high_availability=true`, `my-rack private 2`, each with its id. A row with no name (`None`) is the VPC's main route table, which no subnet Convox created uses.

3. Show the routes on the public route table:

   ```bash
   $ aws ec2 describe-route-tables --route-table-ids rtb-0fedcba9876543210 --query 'RouteTables[0].{routes:Routes,propagating:PropagatingVgws}'
   ```

   Skip the route for the VPC CIDR (`GatewayId` `local`) and the `0.0.0.0/0` route. Copy every other route; these typically have a `TransitGatewayId`, a `VpcPeeringConnectionId`, or a `GatewayId` starting with `vgw-`. A route with `Origin` `EnableVgwRoutePropagation`, or a virtual private gateway listed under `propagating`, comes from VPN route propagation.

4. Add each route to every private route table. For a route to a transit gateway:

   ```bash
   $ aws ec2 create-route --route-table-id rtb-0123456789abcdef0 --destination-cidr-block 10.50.0.0/16 --transit-gateway-id tgw-0123456789abcdef0
   ```

   Use the flag for the route's target:

   | Route target | Flag |
   |--------------|------|
   | Transit gateway | `--transit-gateway-id tgw-0123456789abcdef0` |
   | Virtual private gateway, for Site-to-Site VPN or Direct Connect | `--gateway-id vgw-0123456789abcdef0` |
   | VPC peering connection | `--vpc-peering-connection-id pcx-0123456789abcdef0` |

   A route whose destination is a prefix list takes `--destination-prefix-list-id` in place of `--destination-cidr-block`. For routes that come from VPN route propagation, turn on propagation on each private route table instead:

   ```bash
   $ aws ec2 enable-vgw-route-propagation --route-table-id rtb-0123456789abcdef0 --gateway-id vgw-0123456789abcdef0
   ```

   Repeat for `my-rack private 0`, `my-rack private 1` and, with `high_availability=true`, `my-rack private 2`.

5. If the private subnets have custom network ACLs, allow inbound TCP 443 from the client network and outbound TCP 1024-65535 to it. Convox creates no network ACLs.

Add the routes before turning the parameter on. A self-managed Rack's updates run Terraform on your machine, and Terraform connects to the cluster endpoint directly. If that machine reaches the private endpoint over one of these networks, missing routes stop every later Rack update, including the one that sets the parameter back to `false`, until the routes are added.

### Racks in Existing Subnets

On a Rack installed with `private_subnets_ids` and `public_subnets_ids`, find the route tables your private subnets use:

```bash
$ aws ec2 describe-route-tables --filters Name=association.subnet-id,Values=subnet-0123456789abcdef1,subnet-0123456789abcdef2 --query 'RouteTables[].RouteTableId'
```

A subnet with no explicit association uses the VPC's main route table:

```bash
$ aws ec2 describe-route-tables --filters Name=vpc-id,Values=vpc-0123456789abcdef0 Name=association.main,Values=true --query 'RouteTables[].RouteTableId'
```

Run steps 3 to 5 against those route tables, copying the routes from the route tables of your public subnets.

Check the availability zones and free addresses of the private subnets:

```bash
$ aws ec2 describe-subnets --subnet-ids subnet-0123456789abcdef1 subnet-0123456789abcdef2 --query 'Subnets[].[SubnetId,AvailabilityZone,AvailableIpAddressCount]' --output table
```

Get the subnets the cluster uses now, and run the same `describe-subnets` command on them to see their zones:

```bash
$ aws eks describe-cluster --name my-rack --query cluster.resourcesVpcConfig.subnetIds
```

Every zone of the cluster's current subnets needs at least one private subnet, no private subnet can be in another zone, and each private subnet needs 6 or more free IP addresses (AWS recommends 16).

## Turning It On

```bash
$ convox rack params set eks_control_plane_private_subnets=true -r my-rack
Updating parameters... OK
```

On a Console Rack the update runs as a Rack update job in Console. On a self-managed Rack it runs in the command's output.

The update takes about 5 to 10 minutes. EKS updates the cluster in place, and Apps and `convox` commands for Apps keep working while it runs. The parameter can be set in the same command as other parameters.

To create the cluster in the private subnets from the start, pass the parameter at install:

```bash
$ convox rack install aws my-rack eks_control_plane_private_subnets=true
```

`convox rack install` does not run the checks of `convox rack params set`, so with `private=false` the install fails at plan time with the Terraform error under [Errors](#errors). A Rack installed from Console sets the parameter afterwards with the CLI, because Console's Rack parameter forms do not include it.

## Checking the Result

`convox rack params -g network -r my-rack` lists `eks_control_plane_private_subnets  true`.

The cluster's subnet list holds the ids of the private subnets:

```bash
$ aws eks describe-cluster --name my-rack --query cluster.resourcesVpcConfig.subnetIds
```

The interfaces are in some of the private subnets, not always all of them:

```bash
$ aws ec2 describe-network-interfaces --filters "Name=description,Values=Amazon EKS my-rack" --query 'NetworkInterfaces[].[SubnetId,AvailabilityZone,PrivateIpAddress]' --output table
```

For several minutes after the update finishes, this list can still include interfaces in the old subnets, which EKS then deletes. The `describe-cluster` subnet list is the reliable check.

From a network that uses the private endpoint, `kubectl` commands against the cluster endpoint work.

## New Interface Addresses

The interfaces take addresses from the ranges of the subnets they are in.

| Value | Subnets | Ranges with the default `cidr` of `10.1.0.0/16` |
|-------|---------|--------------------------------------------------|
| `false` | public | `10.1.0.0/20`, `10.1.16.0/20`, `10.1.32.0/20` |
| `true` | private | `10.1.64.0/18`, `10.1.128.0/18`, `10.1.192.0/18` |

The third range in each row exists only with `high_availability=true`. For another `cidr`, public subnets use the `cidr` prefix length plus 4 and private subnets the prefix length plus 2, see [Subnet Layout](/configuration/rack-parameters/aws/cidr#subnet-layout). On a Rack in existing subnets, the addresses come from your private subnets.

Allow the private ranges in any firewall or allowlist, and have clients connect by the endpoint hostname, which stays the same.

## Turning It Off

```bash
$ convox rack params set eks_control_plane_private_subnets=false -r my-rack
Updating parameters... OK
```

The interfaces move back to the public subnets with the same kind of in-place update. Routes you added to the private route tables can stay.

## Errors

| Symptom | Cause | Fix |
|---------|-------|-----|
| `convox rack params set` fails with `ERROR: eks_control_plane_private_subnets requires private subnets, and this rack was installed with private=false` | The Rack has no private subnets | Leave the parameter unset; it is not available on this Rack |
| `convox rack params set` fails with `ERROR: eks_control_plane_private_subnets requires at least 2 private subnets in different availability zones, and private_subnets_ids has 1` (`has 0` when only `public_subnets_ids` was given) | The Rack is in existing subnets with fewer than 2 private subnets | Leave the parameter unset; subnet lists are fixed at install |
| After `--force`, the update fails with `Planning failed` and an `Invalid function argument` error containing `eks_control_plane_private_subnets requires at least 2 private subnets in different availability zones`, on a `private=false` Rack also with `Not enough list items` | `--force` skips the CLI check, and Terraform rejects the value before changing anything | Self-managed Rack: the value stays stored and every later update fails the same way until you run `convox rack params set eks_control_plane_private_subnets=false`. Parameters set in the failed attempts are stored too and apply with that update. Console Rack: only that update job fails, and nothing is stored |
| The update fails with an AWS error from EKS about the subnets or their availability zones | The private subnets of a Rack in existing subnets do not cover the cluster's availability zones | The cluster is unchanged. Set the parameter to `false`; a self-managed Rack keeps the value stored until then, as in the row above |
| `kubectl` over a VPN, peering connection, transit gateway or Direct Connect times out after the change | The return route is only on the public route table | Add it to every private route table, see [Preparing Routes](#preparing-routes), or set the parameter to `false`. A self-managed Rack whose updates run over that network cannot apply `false` until the routes are in place |
| A firewall blocks API traffic after the change | It allows the old addresses | Allow the private subnet ranges, see [New Interface Addresses](#new-interface-addresses) |

## Version Requirements

| Case | Behavior |
|------|----------|
| Rack and CLI | Requires Rack version `3.25.11` or later and CLI version `3.25.11` or later. CLIs `3.24.3` to `3.25.10` reject the parameter with `unknown parameter 'eks_control_plane_private_subnets' for aws provider` |
| Set against a Rack older than `3.25.11` | The update prints `NOTICE: removing parameters not supported by version <version>: eks_control_plane_private_subnets`, nothing moves, and neither a Console Rack nor a self-managed Rack keeps the value. Set it again after updating to `3.25.11` or later |
| Update to `3.25.11` with the parameter unset | No change. The interfaces stay in the public subnets |
| Kubernetes version updates | The interfaces stay in the subnets this parameter selects |
| Downgrade below `3.25.11` with the parameter `true` | The update prints the same `NOTICE` and moves the interfaces back to the public subnets in place. A Console Rack keeps the stored `true`, `convox rack params` still lists it, and the next update to `3.25.11` or later moves the interfaces to the private subnets again. A self-managed Rack drops the value, so set it again after updating |

## See Also

- [private](/configuration/rack-parameters/aws/private) for private subnets behind NAT gateways
- [private_subnets_ids](/configuration/rack-parameters/aws/private_subnets_ids) and [public_subnets_ids](/configuration/rack-parameters/aws/public_subnets_ids) for Racks in existing subnets
- [vpc_id](/configuration/rack-parameters/aws/vpc_id) for Racks in an existing VPC
- [cidr](/configuration/rack-parameters/aws/cidr) for the subnet ranges
- [eks_api_server_private_access_cidrs](/configuration/rack-parameters/aws/eks_api_server_private_access_cidrs) for the security group rules on the interfaces
- [Cluster Endpoint Access](/installation/production-rack/aws#cluster-endpoint-access) for who can reach the API server
- [Direct Kubernetes Access](/management/direct-k8s-access) for `convox rack kubeconfig`
