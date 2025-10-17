---
title: "CLI Rack Management"
description: "Update a v3 rack step-wise through minor versions and manage rack parameters from the convox CLI, including downgrade and reconciliation rules."
slug: cli-rack-management
url: /management/cli-rack-management
---
# CLI Rack Management

## Updating a Rack

### Updating to the latest version
```bash
    $ convox rack update
    Updating rack... OK
```

### Why step-wise updates are required

v3 racks must be updated through every minor version in sequence. Each minor version may include changes to internal rack services, Kubernetes components, or cluster configuration that depend on the previous minor version being applied first. Skipping minor versions can leave these internal components out of sync, which may cause failures that are difficult to recover from.

To update safely, find the latest patch release for each minor version in the [release history](/reference/releases) and update through them one at a time until you reach your target version.

For example, a rack on version `3.21.x` would need to update to the latest `3.22.x` release before proceeding to the latest `3.23.x` release, and so on.

Always update to the **latest patch version** of each minor version. Fixes are applied throughout the lifecycle of a minor release, and skipping to only the `.0` patch can introduce problems that were already resolved in later patches. You do not need to be on the highest patch of your current minor version before updating to the next minor. Go directly to the latest patch of the next minor.

For example, a rack on version `3.22.1` should update directly to the latest `3.23.x` patch release, not to `3.23.0`. Check the [release history](/reference/releases) to find the latest patch for each minor version.

_Note on Versioning: In the `major.minor.patch` format, `minor` versions indicate updates for significant dependencies like Kubernetes, while `patch` versions introduce feature additions or bug fixes._

### Updating to a specific version
```bash
    $ convox rack update 3.23.3
    Updating rack... OK
```

### What happens during an update

When you run `convox rack update`, Convox applies infrastructure changes (Terraform), updates internal services, and may roll Kubernetes components. The rack status changes from `running` to `updating` and back to `running` when complete. Your application containers continue running during the update, because rack updates are designed for zero downtime.

### Downgrading and rolling back

Convox supports rolling a rack back to an earlier **patch** within the same minor version (for example `3.24.8` to `3.24.6`):

```bash
    $ convox rack update 3.24.6
    Updating rack... OK
```

A few guardrails apply:

- **Minor-version downgrades are not supported.** Rolling a v3 rack back across a minor version (for example `3.24.x` to `3.23.x`) is blocked, because each minor version can change Kubernetes components or cluster configuration in ways that cannot be cleanly reversed. The CLI rejects it with `Downgrade from minor version is not supported for v3 rack`. If you need to recover from a bad minor update, contact Convox support: the team can often work out a manual recovery, but it is a hands-on operation, not a self-service rollback. Where possible, prefer rolling **forward** to a newer patch over downgrading.
- **Switch off Contour before downgrading.** If `router_type=contour` is set, a downgrade is blocked with `cannot downgrade while router_type=contour is set`. Set `router_type=nginx` and let the rack finish updating before you downgrade.
- **Parameters are reconciled automatically.** Any rack parameter the older version does not recognize is removed before the apply runs (see [Pre-Apply Reconciliation](#pre-apply-reconciliation) below). Re-apply those parameters after you update forward again.

### Pre-Apply Reconciliation

Convox runs a set of checks immediately before every `terraform apply`, whether the apply comes from an install, a parameter change, or a version update. Each check resolves a condition that is known to fail the apply, and each prints a `NOTICE` to stderr so you can match it against the Rack logs.

| Check | What it does | Example NOTICE |
|-------|--------------|----------------|
| Parameter reconciliation | Removes stored parameters the target version does not declare | `NOTICE: removing parameters not supported by version 3.24.1: fluentd_memory` |
| Stranded Helm release clearing (AWS) | Deletes a Convox-owned Helm revision left pending by an interrupted apply | `NOTICE: cleared stuck Helm release karpenter (pending-upgrade, revision 4) before apply` |
| Additional node group desired size (AWS) | Raises a node group's running size to meet a newly raised `min_size` | `NOTICE: raising node group my-rack-additional-0-a1b2c3d4e5f60718 desired size to 3 before apply` |

**Parameter reconciliation.** Starting with `3.24.2`, Convox reconciles Rack parameters during version transitions. After downloading the target version's Terraform module, Convox scans it for declared variables and compares them against the Rack's stored parameters. Any parameters not accepted by the target version are removed before `terraform apply`, preventing failures from unrecognized arguments. This handles rolling back to a version that predates a parameter, pinning to a release that does not include a recently added parameter, and applying a patch built from an older code base. If no unrecognized parameters are found, the check is a no-op.

**Stranded Helm release clearing.** Starting with `3.25.3`, an apply that was killed while Helm was mid-operation no longer blocks every later apply. Convox deletes the pending revision of the affected Convox-owned release so the next apply can proceed. This runs on AWS Racks only, covers Convox-owned releases only (`aws-lbc`, `karpenter`, `karpenter-crd`, `keda`, `vpa`, `dcgm-exporter`, `nvidia-device-plugin`, `contour`, `contour-internal`, each matched in the namespace Convox installs it into), and acts only on releases stranded for more than fifteen minutes. Helm releases you installed yourself are never touched. Racks whose Kubernetes API is reached through a private endpoint host are covered from `3.25.5`; earlier versions skipped them.

**Additional node group desired size.** Starting with `3.25.3`, raising `min_size` on an entry in `additional_node_groups_config` no longer fails when the pool has autoscaled below the new floor. Convox raises the pool's desired size to the new `min_size` and waits for the scale-up before the apply runs, which makes the update take longer when the increase is large. This runs on AWS Racks only.

**Which version matters.** These checks run in the `convox` CLI, not in the Rack's Terraform modules, so the version that counts is the CLI performing the update: your locally installed `convox` for a self-managed Rack, or the CLI bundled in the Convox Console for a Console-managed Rack. Updating the Rack to a newer version does not deliver them on its own.

If an update fails or the rack remains in `updating` status for an extended period, check the rack logs for errors:

```bash
    $ convox rack logs -r <rack_name>
```

Re-running the same command is safe and is the first thing to try, because the most common stuck-update case, a Helm release stranded by an interrupted apply, is cleared before the next apply runs. Do not stack a second, different update on top of one that is still running. If a re-run fails the same way, contact Convox support with the rack logs. See [Troubleshooting](/help/troubleshooting) for the error text these failures produce.

### Best Practices for Rack Updates

1. **Review the [release notes](https://github.com/convox/convox/releases)** for the target version before updating. Look for breaking changes or special instructions.
2. **Update a staging rack first** to test the new version with your applications before touching production.
3. **Ensure you have recent backups** of critical application data (databases, persistent volumes).
4. **Run updates during a low-traffic window.** Rack updates are designed for zero downtime, but a maintenance window is still recommended for production racks.
5. **Monitor progress** by watching rack logs during the update:
    ```bash
    $ convox rack logs -r <rack_name>
    ```
    The update is complete when the rack status returns to `running`:
    ```bash
    $ convox rack -r <rack_name>
    ```
6. **Update step-wise through minor versions.** A rack on `3.21.x` should update to the latest `3.22.x` before proceeding to `3.23.x`. Never skip minor versions.

## Managing Parameters

Rack parameters control infrastructure-level settings like node sizes, disk allocation, and network configuration. Changing parameters triggers an infrastructure update (similar to a rack version update), so the same caution applies: review changes carefully, test on staging first, and apply during low-traffic windows.

### Viewing current parameters
```bash
    $ convox rack params
    node_disk  100
    node_type  c5.large
```

`convox rack params` lists the parameters that have a value stored for the Rack, not the full set of available parameters. A parameter left at its default does not appear in the output.

### Setting parameters
```bash
    $ convox rack params set node_disk=30 node_type=c5.large
    Updating parameters... OK
```

After running `convox rack params set`, the rack enters an `updating` state while the infrastructure changes are applied. Monitor progress the same way as a version update:

```bash
    $ convox rack logs -r <rack_name>
```

Some parameters (marked with \* in the tables below) can only be set at rack creation time and cannot be changed afterward. Attempting to change them on an existing rack will result in an error.

## Available Parameters

The parameters available for your Rack depend on the underlying cloud provider. The Default column is the value Convox applies when the parameter has never been set. **telemetry** defaults to **false**, but it can be enabled at install time, so run `convox rack params` to see the value stored on a Rack you did not install yourself. It can be changed at any time with `convox rack params set`.

### Amazon Web Services

For detailed descriptions and instructions, visit the [AWS Rack Parameters](/configuration/rack-parameters/aws) page.

| Name                                      | Default                |
|-------------------------------------------|------------------------|
| **access_log_retention_in_days**          | **7**                  |
| **availability_zones**                    |                        |
| **build_node_enabled**                    | **false**              |
| **build_node_min_count**                  | **0**                  |
| **build_node_type**                       |                        |
| **cert_duration**                         | **2160h**              |
| **cidr**                                  | **10.1.0.0/16**        |
| **convox_domain_tls_cert_disable**        | **false**              |
| **efs_csi_driver_enable**                 | **false**              |
| **fluentd_disable**                       | **false**              |
| **gpu_tag_enable**                        | **false**              |
| **high_availability** (1)                 | **true**               |
| **idle_timeout**                          | **3600**               |
| **imds_http_tokens**                      | **optional**           |
| **internal_router**                       | **false**              |
| **karpenter_auth_mode**                   | **false**              |
| **karpenter_enabled**                     | **false**              |
| **internet_gateway_id**                   |                        |
| **max_on_demand_count**                   | **100**                |
| **min_on_demand_count**                   | **1**                  |
| **nlb_security_group**                    |                        |
| **node_capacity_type**                    | **on_demand**          |
| **node_disk**                             | **20**                 |
| **node_type**                             | **t3.small**           |
| **pod_identity_agent_enable**             | **false**              |
| **private**                               | **true**               |
| **private_api**                           | **false**              |
| **private_subnets_ids**                   |                        |
| **proxy_protocol** (2)                    | **false**              |
| **public_subnets_ids**                    |                        |
| **schedule_rack_scale_down**              |                        |
| **schedule_rack_scale_up**                |                        |
| **ssl_ciphers**                           |                        |
| **ssl_protocols**                         |                        |
| **syslog**                                |                        |
| **tags**                                  |                        |
| **telemetry**                             | **false**              |
| **vpc_id** (3)                            |                        |

(1) Parameter cannot be changed after rack creation

(2) Setting **proxy_protocol** in an existing rack will require a 5 - 10 minutes downtime window.

(3) To avoid CIDR block collision with existing VPC subnets, add a new CIDR block to your VPC to separate rack resources. Also, remember to pass the **internet_gateway_id** attached to the VPC. If the VPC doesn't have an IG attached, the rack installation will create one automatically, which will also be destroyed if you delete the rack.

> **schedule_rack_scale_down** and **schedule_rack_scale_up** are mutually exclusive. You must set both for scheduled scale operations. If you set only **schedule_rack_scale_down**, the rack will not scale up on its own.

### Digital Ocean

For detailed descriptions and instructions, visit the [Digital Ocean Rack Parameters](/configuration/rack-parameters/do) page.

| Name                    | Default           |
|-------------------------|-------------------|
| **cert_duration**       | **2160h**         |
| **node_type**           | **s-2vcpu-4gb**   |
| **region**              | **nyc3**          |
| **registry_disk**       | **50Gi**          |
| **syslog**              |                   |
| **high_availability** * | **true**          |
| **telemetry**           | **false**         |

\* Parameter cannot be changed after rack creation

### Google Cloud Platform

For detailed descriptions and instructions, visit the [Google Cloud Platform Rack Parameters](/configuration/rack-parameters/gcp) page.

| Name                    | Default           |
|-------------------------|-------------------|
| **cert_duration**       | **2160h**         |
| **node_type**           | **n1-standard-2** |
| **preemptible**         | **true**          |
| **region**              | **us-east1**      |
| **syslog**              |                   |
| **telemetry**           | **false**         |

### Microsoft Azure

For detailed descriptions and instructions, visit the [Microsoft Azure Rack Parameters](/configuration/rack-parameters/azure) page.

| Name                    | Default           |
|-------------------------|-------------------|
| **cert_duration**       | **2160h**         |
| **node_type**           | **Standard_D2_v3**|
| **region**              | **eastus**        |
| **syslog**              |                   |
| **telemetry**           | **false**         |

## See Also

- [Console Rack Management](/management/console-rack-management) for managing racks through the web console
- [Rack Parameters](/configuration/rack-parameters) for configuring rack settings
