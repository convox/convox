---
title: "GCP Rack Parameters"
slug: gcp-rack-parameters
url: /configuration/rack-parameters/gcp
---
# GCP Rack Parameters

The following parameters are available for configuring your Convox rack on Google Cloud Platform (GCP). These parameters allow you to customize and optimize the behavior of your applications and services running on the GCP platform.

> Some parameters can only be set during rack installation and cannot be changed afterwards. These include `region`. See individual parameter pages for details.

## Parameters

| Parameter                            | Description                                                              |
|:-------------------------------------|:-------------------------------------------------------------------------|
| [cert_duration](/configuration/rack-parameters/gcp/cert_duration)         | Certificate renewal period.                                               |
| [docker_hub_password](/configuration/rack-parameters/gcp/docker_hub_password) | Docker Hub access token for authenticated image pulls. |
| [docker_hub_username](/configuration/rack-parameters/gcp/docker_hub_username) | Docker Hub username for authenticated image pulls. |
| [node_disk](/configuration/rack-parameters/gcp/node_disk)                 | Size of the root disk (in GB) for each node.                              |
| [node_type](/configuration/rack-parameters/gcp/node_type)                 | Specifies the node instance type.                                         |
| [preemptible](/configuration/rack-parameters/gcp/preemptible)             | Use preemptible instances for cost savings.                               |
| [region](/configuration/rack-parameters/gcp/region)                       | Specifies the GCP region for the rack.                                    |
| [syslog](/configuration/rack-parameters/gcp/syslog)                       | Specifies the endpoint to forward logs to a syslog server.                |
