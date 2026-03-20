---
title: "CLI Rack Management"
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

Always update to the **latest patch version** of each minor version. Fixes are applied throughout the lifecycle of a minor release, and skipping to only the `.0` patch can introduce problems that were already resolved in later patches. You do not need to be on the highest patch of your current minor version before updating to the next minor -- go directly to the latest patch of the next minor.

For example, a rack on version `3.22.1` should update directly to `3.23.3` (the latest version in `3.23.x` at the time of this writing), not to `3.23.0`.

_Note on Versioning: In the `major.minor.patch` format, `minor` versions indicate updates for significant dependencies like Kubernetes, while `patch` versions introduce feature additions or bug fixes._

### Updating to a specific version
```bash
    $ convox rack update 3.23.3
    Updating rack... OK
```

### What happens during an update

When you run `convox rack update`, Convox applies infrastructure changes (Terraform), updates internal services, and may roll Kubernetes components. The rack status changes from `running` to `updating` and back to `running` when complete. Your application containers continue running during the update -- rack updates are designed for zero downtime.

If an update fails or the rack remains in `updating` status for an extended period, check the rack logs for errors:

```bash
    $ convox rack logs -r <rack_name>
```

If you encounter a stuck update, contact Convox support with the rack logs. Do not attempt to force another update on top of a failed one.

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
    node_disk  20
    node_type  t3.small
```
### Setting parameters
```bash
    $ convox rack params set node_disk=30 node_type=c5.large
    Setting parameters... OK
```

After running `convox rack params set`, the rack enters an `updating` state while the infrastructure changes are applied. Monitor progress the same way as a version update:

```bash
    $ convox rack logs -r <rack_name>
```

Some parameters (marked with \* in the tables below) can only be set at rack creation time and cannot be changed afterward. Attempting to change them on an existing rack will result in an error.

## Available Parameters

The parameters available for your Rack depend on the underlying cloud provider.

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
| **internet_gateway_id**                   |                        |
| **max_on_demand_count**                   | **100**                |
| **min_on_demand_count**                   | **1**                  |
| **nlb_security_group**                    |                        |
| **node_capacity_type**                    | **on_demand**          |
| **node_disk**                             | **20**                 |
| **node_type**                             | **t3.small**           |
| **pod_identity_agent_enable**             | **false**              |
| **private**                               | **true**               |
| **private_subnets_ids**                   |                        |
| **proxy_protocol** (2)                    | **false**              |
| **public_subnets_ids**                    |                        |
| **schedule_rack_scale_down**              |                        |
| **schedule_rack_scale_up**                |                        |
| **ssl_ciphers**                           |                        |
| **ssl_protocols**                         |                        |
| **syslog**                                |                        |
| **tags**                                  |                        |
| **telemetry**                             | **true**               |
| **vpc_id** (3)                            |                        |

(1) Parameter cannot be changed after rack creation

(2) Setting **proxy_protocol** in an existing rack will require a 5 - 10 minutes downtime window.

(3) To avoid CIDR block collision with existing VPC subnets, add a new CIDR block to your VPC to separate rack resources. Also, remember to pass the **internet_gateway_id** attached to the VPC. If the VPC doesn't have an IG attached, the rack installation will create one automatically, which will also be destroyed if you delete the rack.

> **schedule_rack_scale_down** and **schedule_rack_scale_up** are mutually exclusive. You must set both for scheduled scale operations. If you set only **schedule_rack_scale_down**, the rack will not scale up on its own.

&nbsp;

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
| **telemetry**           | **true**          |

\* Parameter cannot be changed after rack creation

&nbsp;

### Google Cloud Platform

For detailed descriptions and instructions, visit the [Google Cloud Platform Rack Parameters](/configuration/rack-parameters/gcp) page.

| Name                    | Default           |
|-------------------------|-------------------|
| **cert_duration**       | **2160h**         |
| **node_type**           | **n1-standard-1** |
| **preemptible**         | **true**          |
| **region**              | **us-east1**      |
| **syslog**              |                   |
| **telemetry**           | **true**          |

&nbsp;

### Microsoft Azure

For detailed descriptions and instructions, visit the [Microsoft Azure Rack Parameters](/configuration/rack-parameters/azure) page.

| Name                    | Default           |
|-------------------------|-------------------|
| **cert_duration**       | **2160h**         |
| **node_type**           | **Standard_D3_v3**|
| **region**              | **eastus**        |
| **syslog**              |                   |
| **telemetry**           | **true**          |

## See Also

- [Console Rack Management](/management/console-rack-management) for managing racks through the web console
- [Rack Parameters](/configuration/rack-parameters) for configuring rack settings
