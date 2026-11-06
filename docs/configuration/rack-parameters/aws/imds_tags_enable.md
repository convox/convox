---
title: "imds_tags_enable"
description: "The imds_tags_enable AWS rack parameter exposes EC2 instance tags through the Instance Metadata Service on Rack nodes, defaulting to false."
slug: imds_tags_enable
url: /configuration/rack-parameters/aws/imds_tags_enable
---

# imds_tags_enable

## Description
The `imds_tags_enable` parameter controls whether EC2 exposes an instance's tags through the Instance Metadata Service (IMDS) on Rack nodes. When enabled, processes running on a node can read that instance's tags at `http://169.254.169.254/latest/meta-data/tags/instance` without calling the EC2 API and without the `ec2:DescribeTags` IAM permission.

The parameter sets the `instance_metadata_tags` option on the launch templates for the EKS managed node groups the Rack creates: the main node group, the build node group, and any additional node groups or additional build node groups. It does not apply to Karpenter-provisioned nodes, which launch with instance metadata tags disabled.

Convox tags Rack node instances with `Name` and `Rack` (both set to the Rack name) plus any tags supplied through the [tags](/configuration/rack-parameters/aws/tags) parameter and, for additional node groups, any per-node-group tags. Enabling this parameter makes those tags readable from inside the instance.

This parameter is available on AWS Racks since Rack version `3.17.0`.

## Default Value
The default value for `imds_tags_enable` is `false`.

## Use Cases
- **Node Bootstrap**: Scripts supplied through `user_data` can read the Rack name or environment tags from IMDS during startup instead of calling the EC2 API.
- **Monitoring and Logging Agents**: Agents running on nodes can label metrics and logs with instance tags without EC2 API calls or additional IAM permissions.
- **API Throttling Avoidance**: On larger node fleets, reading tags from IMDS avoids `ec2:DescribeTags` API rate limits.

## Setting Parameters
To enable instance metadata tags, use the following command:
```bash
$ convox rack params set imds_tags_enable=true -r rackName
Updating parameters... OK
```

Applying a change to this parameter creates a new launch template version, and EKS performs a rolling replacement of the node group instances.

## Additional Information
- When instance metadata tags are enabled, EC2 only accepts tag keys containing letters, numbers, and the characters `+ - = . , _ : @`. Instance launches fail if a tag key contains spaces, `/`, or other unsupported characters. Verify the keys in the `tags` parameter and in any additional node group tags before enabling.
- Tags are served through the same IMDS endpoint as other instance metadata, so the [imds_http_tokens](/configuration/rack-parameters/aws/imds_http_tokens) and [imds_http_hop_limit](/configuration/rack-parameters/aws/imds_http_hop_limit) settings apply to tag reads as well.
- Any Process that can reach IMDS can read the exposed tags. If your tags carry information that App workloads should not see, leave this parameter disabled or block pod access to IMDS with [pod_imds_block_enabled](/configuration/rack-parameters/aws/pod_imds_block_enabled).
- On Racks with `high_availability=false`, this parameter cannot be changed in the same `convox rack params set` call that sets `karpenter_enabled=true`. Set it first, wait for the update to complete, then enable Karpenter.

## See Also
- [imds_http_tokens](/configuration/rack-parameters/aws/imds_http_tokens)
- [imds_http_hop_limit](/configuration/rack-parameters/aws/imds_http_hop_limit)
- [pod_imds_block_enabled](/configuration/rack-parameters/aws/pod_imds_block_enabled)
- [tags](/configuration/rack-parameters/aws/tags)
