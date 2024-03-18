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

E.g. a rack on Version `3.12.x` would need update to the latest `3.13.x` version before proceeding to the latest `3.15.x` version and so on.

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

| Name                              | Default         |
|---------------------------------- |-----------------|
| **cidr**                          | **10.1.0.0/16** |
| **internet_gateway_id**           |                 |
| **internal_router**               | **false**       |
| **node_capacity_type**            | **on_demand**   |
| **node_disk**                     | **20**          |
| **node_type**                     | **t3.small**    |
| **region**                        | **us-east-1**   |
| **high_availability** *           | **true**        |
| **schedule_rack_scale_down**      |                 |
| **schedule_rack_scale_up**        |                 |
| **proxy_protocol** **             | **false**       |
| **vpc_id** ***                    |                 |
| **ssl_ciphers**                   |                 |
| **ssl_protocols**                 |                 |
| **tags**                          |                 |
| **telemetry**                     | **true**        |

\* Parameter cannot be changed after rack creation

\*\* Setting **proxy_protocol** in an existing rack will require a 5 - 10 minutes downtime window.

\*\*\* To avoid CIDR block collision with existing VPC subnets, please add a new CIDR block to your VPC to separate rack resources. Also, remember to pass the **internet_gateway_id** attached to the VPC. If the VPC doesn't have an IG attached, the rack installation will create one automatically, which will also be destroyed if you delete the rack.

\*\*\* **schedule_rack_scale_down** and **schedule_rack_scale_up** are mutually exclusive. So you have to set both of them properly for the scheduled timed off. If you set only **schedule_rack_scale_down**, it will not scale up on its own.

&nbsp;

### Digital Ocean

| Name                    | Default           |
|-------------------------|-------------------|
| **node_type**           | **s-2vcpu-4gb**   |
| **region**              | **nyc3**          |
| **registry_disk**       | **50Gi**          |
| **high_availability** * | **true**          |
| **telemetry**           | **true**          |

\* Parameter cannot be changed after rack creation

&nbsp;

### Google Cloud

| Name        | Default         |
| ----------- | --------------- |
| **node_type** | **n1-standard-1** |
| **telemetry** | **true**        |

&nbsp;

### Microsoft Azure

| Name        | Default          |
| ----------- | ---------------- |
| **node_type** | **Standard_D3_v3** |
| **region**    | **eastus**         |
| **telemetry** | **true**        |
