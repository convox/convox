---
title: "Import a Rack into Console"
slug: import-rack
url: /management/import-rack
---
# Import a Rack into Console

A Rack installed from the CLI is managed locally. Terraform state lives on the machine that created it, and only that machine can update it. Importing the Rack into the Console transfers state management to your organization so any team member with access can manage, update, and interact with the Rack.

## Prerequisites

- A Convox Console account. [Sign up](https://console.convox.com/signup) if you do not have one.
- An organization in the Console. Use your company name as the organization name.
- The CLI logged in to your Console account (`convox login`).
- A locally installed Rack in `running` status.
- For AWS Racks: access to the AWS IAM console for the target account.

## Check Current Rack Status

Verify the Rack is locally managed (no organization prefix) and running:

```bash
$ convox racks
```

Output for a locally managed Rack:

```text
NAME               PROVIDER  STATUS
staging            aws       running
```

## Move the Rack to Console

Use `convox rack mv` to transfer the Rack. The destination uses the format `<organization>/<rack-name>`:

```bash
$ convox rack mv staging acme/staging
```

The Rack name must remain the same. Only the organization prefix changes.

After the move completes, verify:

```bash
$ convox racks
```

```text
NAME               PROVIDER  STATUS
acme/staging       aws       running
```

The Rack now appears in the Console under the `acme` organization. Team members with access to the organization can see and manage the Rack from their own CLI or from the Console.

## AWS Racks: Additional IAM Step

AWS Racks require an additional step to grant the Console role access to the EKS cluster. This is due to how AWS manages Kubernetes authentication through the `aws-auth` ConfigMap.

1. In the AWS IAM console, find the ConsoleRole ARN. It looks like:
   ```text
   arn:aws:iam::ACCOUNT_ID:role/convox-ORG_ID-ConsoleRole-XXXXXXXXXX
   ```
   If the ARN contains an extra `convox/` path prefix between `role/` and the role name, omit that prefix.

2. Get `kubectl` access to the Rack:
   ```bash
   $ convox rack kubeconfig -r rackName > ~/.kube/config
   ```

3. Save a backup of the current ConfigMap:
   ```bash
   $ kubectl get configmap/aws-auth -n kube-system -o yaml > aws-auth-backup.yaml
   ```

4. Edit the `aws-auth` ConfigMap:
   ```bash
   $ kubectl edit configmap/aws-auth -n kube-system
   ```

5. Add a new entry to `mapRoles`:
   ```yaml
   - rolearn: arn:aws:iam::ACCOUNT_ID:role/convox-ORG_ID-ConsoleRole-XXXXXXXXXX
     username: convox-console
     groups:
     - system:masters
   ```

**Warning:** Editing the `aws-auth` ConfigMap incorrectly can lock users out of the cluster. Proceed with caution and verify the existing entries before saving.

GCP, Azure, and DigitalOcean Racks do not require this step.

## What Changes After Import

| Aspect | Before (CLI-managed) | After (Console-managed) |
|--------|---------------------|------------------------|
| Terraform state | Stored locally on your machine | Stored in the Console |
| Updates | Only the installing machine can update | Any team member with access can update |
| Visibility | Only visible to the local CLI user | Visible to all organization members |
| Console features | Not available | Full Console access: deploy workflows, RBAC, audit logs, metrics dashboard, GPU telemetry, cost tracking |

## Moving a Rack Back to CLI

Transfer a Console-managed Rack back to local management:

```bash
$ convox rack mv acme/staging staging
```

Terraform state transfers to your local machine for exclusive management.

## Limitations

- The Rack name cannot change during the move. `convox rack mv staging acme/production` is not supported.
- A Rack with active dependencies (in-progress deploys or updates) cannot be moved until those operations complete.
- After importing an AWS Rack, the IAM step above must be completed before the Console can manage the Rack.

## See Also

- [Console Rack Management](/management/console-rack-management) for full details on Console vs. CLI management
- [CLI Rack Management](/management/cli-rack-management) for managing Racks from the command line
- [RBAC](/management/rbac) for configuring team access controls
