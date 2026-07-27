---
title: "GCP Rack Parameters"
description: "Reference for the rack parameters available when running a Convox rack on Google Cloud Platform, covering node type, disk, region, preemptible nodes, and logging."
slug: gcp-rack-parameters
url: /configuration/rack-parameters/gcp
---
# GCP Rack Parameters

The following parameters are available for configuring your Convox rack on Google Cloud Platform (GCP). These parameters allow you to customize and optimize the behavior of your applications and services running on the GCP platform.

> Some parameters can only be set during rack installation and cannot be changed afterwards. These include `region`. See individual parameter pages for details.

## Parameters

| Parameter                            | Description                                                              |
|:-------------------------------------|:-------------------------------------------------------------------------|
| [additional_node_groups_config](/configuration/rack-parameters/gcp/additional_node_groups_config) | Configures additional customized node pools for the cluster, including GPU pools and single-host Cloud TPU pools. |
| [cert_duration](/configuration/rack-parameters/gcp/cert_duration)         | Certificate renewal period.                                               |
| [dcgm_scrape_interval](/configuration/rack-parameters/gcp/dcgm_scrape_interval) | Prometheus scrape interval hint annotated on the DCGM exporter for GPU metrics. |
| [docker_hub_password](/configuration/rack-parameters/gcp/docker_hub_password) | Docker Hub access token for authenticated image pulls. |
| [docker_hub_username](/configuration/rack-parameters/gcp/docker_hub_username) | Docker Hub username for authenticated image pulls. |
| [fluentd_memory](/configuration/rack-parameters/gcp/fluentd_memory)       | Configures memory allocation for the Fluentd log collector DaemonSet.     |
| [gpu_observability_chart_version](/configuration/rack-parameters/gcp/gpu_observability_chart_version) | Pins the Helm chart version for the NVIDIA DCGM exporter. |
| [gpu_observability_enable](/configuration/rack-parameters/gcp/gpu_observability_enable) | Installs the NVIDIA DCGM exporter to emit GPU telemetry for a Prometheus you run yourself. |
| [nginx_additional_config](/configuration/rack-parameters/gcp/nginx_additional_config) | Passes additional key-value configuration pairs to the nginx ingress controller ConfigMap. |
| [node_disk](/configuration/rack-parameters/gcp/node_disk)                 | Size of the root disk (in GB) for each node.                              |
| [node_type](/configuration/rack-parameters/gcp/node_type)                 | Specifies the node instance type.                                         |
| [preemptible](/configuration/rack-parameters/gcp/preemptible)             | Use preemptible instances for cost savings.                               |
| [region](/configuration/rack-parameters/gcp/region)                       | Specifies the GCP region for the rack.                                    |
| [syslog](/configuration/rack-parameters/gcp/syslog)                       | Specifies the endpoint to forward logs to a syslog server.                |
| [terraform_update_timeout](/configuration/rack-parameters/gcp/terraform_update_timeout) | Controls how long Terraform waits for node pool update operations to complete. |
| [webhook_signing_key](/configuration/rack-parameters/gcp/webhook_signing_key) | Per-rack HMAC secret that signs outbound webhook deliveries with a Convox-Signature header. |
