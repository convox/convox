---
title: "key_pair_name"
slug: key_pair_name
url: /configuration/rack-parameters/aws/key_pair_name
---

# key_pair_name

## Description
The `key_pair_name` parameter specifies an AWS EC2 Key Pair to install on all EKS cluster node instances. When set, the specified key pair's public key is injected into the launch template for all node groups (primary, build, and additional), enabling SSH access to the underlying EC2 instances.

This parameter works with the `convox instances keyroll` command, which generates a new key pair and automatically updates this parameter.

## Default Value
The default value for `key_pair_name` is an empty string (no key pair assigned).

When not set, no SSH key is installed on the cluster nodes, which effectively disables direct SSH access to the instances.

## Use Cases
- **Debugging**: SSH into nodes to diagnose issues that cannot be resolved through Kubernetes tooling alone (e.g., disk pressure, networking problems, kubelet issues).
- **Compliance**: Some security policies require the ability to access underlying infrastructure for audit or incident response.
- **Key Rotation**: Use `convox instances keyroll` to periodically rotate the SSH key pair. The command generates a new key pair, updates this parameter, and outputs the new private key.

## Setting Parameters
To assign an existing EC2 key pair to your rack nodes:
```bash
$ convox rack params set key_pair_name=my-keypair -r rackName
Setting parameters... OK
```

To rotate the key pair using the CLI:
```bash
$ convox instances keyroll -r rackName
```

The `keyroll` command generates a new key pair, sets `key_pair_name` to the new key pair name, and prints the private key. Save the private key immediately as it cannot be retrieved again.

## Additional Information
- The key pair is applied to all node groups in the rack: primary nodes, build nodes, and any additional node groups.
- Changing this parameter triggers a node group update, which will roll the nodes according to your [node_max_unavailable_percentage](/configuration/rack-parameters/aws/node_max_unavailable_percentage) setting.
- The key pair must already exist in the same AWS region as the rack, unless using `convox instances keyroll` which creates the key pair automatically.
- To remove SSH access, set the parameter back to an empty string: `convox rack params set key_pair_name= -r rackName`.

## See Also
- [node_max_unavailable_percentage](/configuration/rack-parameters/aws/node_max_unavailable_percentage) for controlling node update behavior
- [instances](/reference/cli/instances) for the `instances keyroll` command
