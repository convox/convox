---
title: "Oracle Cloud Rack Parameters"
description: "Reference for the rack parameters available when running a Convox rack on Oracle Cloud Infrastructure, covering node shape, disk, GPU node pools, region, and logging."
slug: oci-rack-parameters
url: /configuration/rack-parameters/oci
---
# Oracle Cloud Rack Parameters

The following parameters are available for configuring your Convox rack on Oracle Cloud Infrastructure (OCI). These parameters allow you to customize and optimize the behavior of your applications and services running on the OCI platform.

> Some parameters can only be set during rack installation and cannot be changed afterwards. These include `region`. See individual parameter pages for details.

## Parameters

| Parameter                            | Description                                                              |
|:-------------------------------------|:-------------------------------------------------------------------------|
| [cert_duration](/configuration/rack-parameters/oci/cert_duration)         | Certificate renewal period.                                               |
| [compartment_ocid](/configuration/rack-parameters/oci/compartment_ocid)   | OCI compartment where rack resources are created.                        |
| [docker_hub_password](/configuration/rack-parameters/oci/docker_hub_password) | Docker Hub access token for authenticated image pulls. |
| [docker_hub_username](/configuration/rack-parameters/oci/docker_hub_username) | Docker Hub username for authenticated image pulls. |
| [fluentd_memory](/configuration/rack-parameters/oci/fluentd_memory)       | Configures memory allocation for the Fluentd log collector DaemonSet.     |
| [gpu_node_count](/configuration/rack-parameters/oci/gpu_node_count)       | Number of nodes in the optional GPU node pool.                            |
| [gpu_node_type](/configuration/rack-parameters/oci/gpu_node_type)         | GPU compute shape for an optional tainted GPU node pool.                  |
| [high_availability](/configuration/rack-parameters/oci/high_availability) | Enable high availability mode with multiple nodes.                        |
| [node_count](/configuration/rack-parameters/oci/node_count)               | Number of nodes in the rack's node pool.                                  |
| [node_disk](/configuration/rack-parameters/oci/node_disk)                 | Size of the node boot volume in GB.                                       |
| [node_memory](/configuration/rack-parameters/oci/node_memory)             | Memory per node in GB, for Flex compute shapes.                           |
| [node_ocpus](/configuration/rack-parameters/oci/node_ocpus)               | OCPUs per node, for Flex compute shapes.                                  |
| [node_type](/configuration/rack-parameters/oci/node_type)                 | Specifies the node compute shape.                                         |
| [region](/configuration/rack-parameters/oci/region)                       | Specifies the OCI region for the rack.                                    |
| [syslog](/configuration/rack-parameters/oci/syslog)                       | Specifies the endpoint to forward logs to a syslog server.                |
| [terraform_update_timeout](/configuration/rack-parameters/oci/terraform_update_timeout) | Controls how long Terraform waits for node pool update operations to complete. |
| [webhook_signing_key](/configuration/rack-parameters/oci/webhook_signing_key) | Per-rack HMAC secret that signs outbound webhook deliveries with a Convox-Signature header. |
