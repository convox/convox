---
title: "eks_access_entries"
slug: eks_access_entries
url: /configuration/rack-parameters/aws/eks_access_entries
---

# eks_access_entries

## Description

The `eks_access_entries` parameter creates EKS Access Entries for the IAM role managing the rack and the nodes IAM role. This enables migration from the legacy `aws-auth` ConfigMap to the EKS Access Entries API for cluster authentication.

When enabled, the parameter:

| Action | Detail |
|--------|--------|
| Auth mode switch | Sets EKS authentication mode to `API_AND_CONFIG_MAP` (idempotent if already set by `karpenter_auth_mode`) |
| Caller access entry | Creates a `STANDARD` type entry for the Terraform caller's IAM role with `AmazonEKSClusterAdminPolicy` |
| Nodes access entry | Creates an `EC2_LINUX` type entry for the nodes IAM role, ensuring node registration works without `aws-auth` |

This parameter is **one-way**: it cannot be disabled once enabled. The CLI rejects `eks_access_entries=false` when the current value is `true`.

## Default Value

The default value for `eks_access_entries` is `false`.

## Why Migrate

The `aws-auth` ConfigMap approach to EKS authentication:
- Can be accidentally deleted, locking users out of the cluster
- Requires Kubernetes API access to modify
- Lacks CloudTrail audit logging

EKS Access Entries operate at the AWS API layer, are AWS-managed, provide CloudTrail audit logging, and cannot be disrupted by Kubernetes-level operations.

## Setting Parameters

```bash
$ convox rack params set eks_access_entries=true -r rackName
Setting parameters... OK
```

After the update completes, verify the entries exist:

```bash
$ aws eks list-access-entries --cluster-name <cluster-name> --region <region>
```

## Interaction with Karpenter

If your rack already has `karpenter_auth_mode=true`, the EKS auth mode is already `API_AND_CONFIG_MAP`. Enabling `eks_access_entries` adds the caller and nodes access entries without re-running the auth mode switch.

| `karpenter_auth_mode` | `eks_access_entries` | Auth mode switch | Karpenter node entry | Caller access entry | Nodes access entry |
|---|---|---|---|---|---|
| false | false | No | No | No | No |
| true | false | Yes | Yes | No | No |
| false | true | Yes | No | Yes | Yes |
| true | true | Yes | Yes | Yes | Yes |

## Downgrade Considerations

Downgrading to a rack version without `eks_access_entries` is safe only if you have not deleted the `aws-auth` ConfigMap:

| User action after enabling | Downgrade safe? | Detail |
|----------------------------|-----------------|--------|
| No changes (kept `aws-auth`, kept `API_AND_CONFIG_MAP`) | Yes | Access entries persist in AWS. Authentication falls back to `aws-auth`. |
| Deleted `aws-auth` ConfigMap | Risky | Access entries persist but are no longer managed. If entries are removed (manually, by cleanup script, or AWS), there is no automated recovery and no `aws-auth` fallback. |
| Switched cluster to `API` only mode | Not safe | `aws-auth` is disabled under `API` mode. Loss of access entries means loss of cluster access. |

**Recommendation:** Do not delete the `aws-auth` ConfigMap or switch to `API` only mode unless you are committed to staying on 3.24.7 or later.

## Additional Information

- All shell provisioners check for existing entries before creating, handling concurrent operations gracefully.
- The caller access entry uses `data.aws_iam_session_context` to resolve the IAM role from the current caller identity, ensuring the correct role receives the entry regardless of assumed-role session variations.
- The nodes access entry uses the `EC2_LINUX` type, which grants node bootstrap permissions without requiring an explicit policy association.

## See Also

- [karpenter_auth_mode](/configuration/rack-parameters/aws/karpenter_auth_mode) for the Karpenter-specific auth mode migration
- [Karpenter](/configuration/scaling/karpenter) for Karpenter node autoscaling configuration
- [direct-k8s-access](/management/direct-k8s-access) for direct Kubernetes access to Convox-managed clusters
