---
title: "CLI Rack Management"
draft: false
slug: CLI Rack Management
url: /management/cli-rack-management
---
# CLI Rack Management

## Updating a Rack

### Updating to the latest version
```html
    $ convox rack update
    Updating rack... OK
```

v3 racks need to be updated through every minor version.  We suggest you [locate the latest minor rack version](https://github.com/convox/convox/releases) that you are updating through and then continuing up versions "step-wise" until you reach your desired version.  This is to ensure no internal rack or cluster services fall out of sync/version with each other.  

E.g. a rack on Version `3.13.x` would need update to the latest `3.14.x` version before proceeding to the latest `3.15.x` version and so on.

You should always update to the latest patch version of your new version because often times fixes are applied throughout the minor which can cause problems if going to only the base version. Additionally you do not need to be on the highest patch version of your current minor to update your rack to the next minor.

E.g. a rack on Version `3.13.1` should update directly to `3.14.5` (the latest version in `3.14.x` at the time of this writing).

_Note on Versioning: In the `major.minor.patch` format, `minor` versions indicate updates for significant dependencies like Kubernetes, while `patch` versions introduce feature additions or bug fixes._

### Updating to a specific version
```html
    $ convox rack update 3.16.1
    Updating rack... OK
```

## Managing Parameters

### Viewing current parameters
```html
    $ convox rack params
    node_disk  20
    node_type  t3.small
```
### Setting parameters
```html
    $ convox rack params set node_disk=30 node_type=c5.large
    Setting parameters... OK
```

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
| **high_availability** *                   | **true**               |
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
| **proxy_protocol** **                     | **false**              |
| **public_subnets_ids**                    |                        |
| **schedule_rack_scale_down**              |                        |
| **schedule_rack_scale_up**                |                        |
| **ssl_ciphers**                           |                        |
| **ssl_protocols**                         |                        |
| **syslog**                                |                        |
| **tags**                                  |                        |
| **telemetry**                             | **true**               |
| **vpc_id** ***                            |                        |

\* Parameter cannot be changed after rack creation

\*\* Setting **proxy_protocol** in an existing rack will require a 5 - 10 minutes downtime window.

\*\*\* To avoid CIDR block collision with existing VPC subnets, please add a new CIDR block to your VPC to separate rack resources. Also, remember to pass the **internet_gateway_id** attached to the VPC. If the VPC doesn't have an IG attached, the rack installation will create one automatically, which will also be destroyed if you delete the rack.

\*\*\* **schedule_rack_scale_down** and **schedule_rack_scale_up** are mutually exclusive. So you have to set both of them properly for the scheduled timed off. If you set only **schedule_rack_scale_down**, it will not scale up on its own.

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
