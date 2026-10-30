---
title: "GCP Rack Parameters"
description: "Reference for the rack parameters of a Convox rack on Google Cloud Platform, covering node type, disk, region, preemptible nodes, cost tracking, and logging."
slug: gcp-rack-parameters
url: /configuration/rack-parameters/gcp
---
# GCP Rack Parameters

The following parameters are available for configuring your Convox rack on Google Cloud Platform (GCP). These parameters allow you to customize and optimize the behavior of your applications and services running on the GCP platform.

> Some parameters can only be set during rack installation and cannot be changed afterwards. These include `region`. See individual parameter pages for details.

## Parameters

| Parameter                            | Description                                                              |
|:-------------------------------------|:-------------------------------------------------------------------------|
| [additional_node_groups_config](/configuration/rack-parameters/gcp/additional_node_groups_config) | Configures additional customized node pools for the cluster, including GPU pools, single-host Cloud TPU pools and Arm pools. |
| [cert_duration](/configuration/rack-parameters/gcp/cert_duration)         | Certificate renewal period.                                               |
| [cost_tracking_enable](/configuration/rack-parameters/gcp/cost_tracking_enable) | Turns on the rack-side cost accumulator that powers `convox cost` and per-app budget caps. |
| [cost_tracking_history_days](/configuration/rack-parameters/gcp/cost_tracking_history_days) | Sets how many days of daily App cost history the Rack keeps for `convox cost --start` and `--end`. |
| [dcgm_scrape_interval](/configuration/rack-parameters/gcp/dcgm_scrape_interval) | Prometheus scrape interval hint annotated on the DCGM exporter for GPU metrics. |
| [docker_hub_password](/configuration/rack-parameters/gcp/docker_hub_password) | Docker Hub access token for authenticated image pulls. |
| [docker_hub_username](/configuration/rack-parameters/gcp/docker_hub_username) | Docker Hub username for authenticated image pulls. |
| [elasticsearch_disk](/configuration/rack-parameters/gcp/elasticsearch_disk) | Size of each Elasticsearch log volume, which sets how much log history the Rack keeps. |
| [filestore_enable](/configuration/rack-parameters/gcp/filestore_enable)   | Enables Google Cloud Filestore NFS volumes for shared persistent storage. |
| [fluentd_memory](/configuration/rack-parameters/gcp/fluentd_memory)       | Configures memory allocation for the Fluentd log collector DaemonSet.     |
| [gpu_observability_chart_version](/configuration/rack-parameters/gcp/gpu_observability_chart_version) | Pins the Helm chart version for the NVIDIA DCGM exporter. |
| [gpu_observability_enable](/configuration/rack-parameters/gcp/gpu_observability_enable) | Installs the NVIDIA DCGM exporter to emit GPU telemetry for a Prometheus you run yourself. |
| [nginx_additional_config](/configuration/rack-parameters/gcp/nginx_additional_config) | Passes additional key-value configuration pairs to the nginx ingress controller ConfigMap. |
| [node_disk](/configuration/rack-parameters/gcp/node_disk)                 | Size of the root disk (in GB) for each node.                              |
| [node_type](/configuration/rack-parameters/gcp/node_type)                 | Specifies the node instance type.                                         |
| [preemptible](/configuration/rack-parameters/gcp/preemptible)             | Use preemptible instances for cost savings.                               |
| [region](/configuration/rack-parameters/gcp/region)                       | Specifies the GCP region for the rack.                                    |
| [router_log_redact_params](/configuration/rack-parameters/gcp/router_log_redact_params) | Masks query parameter values in router access and error logs, for every parameter (`all`) or a list of names. |
| [syslog](/configuration/rack-parameters/gcp/syslog)                       | Specifies the endpoint to forward logs to a syslog server.                |
| [syslog_tls_verify](/configuration/rack-parameters/gcp/syslog_tls_verify) | Verifies the certificate of a `tcp+tls` syslog endpoint against public certificate authorities. |
| [terraform_update_timeout](/configuration/rack-parameters/gcp/terraform_update_timeout) | Controls how long Terraform waits for node pool update operations to complete. |
| [webhook_signing_key](/configuration/rack-parameters/gcp/webhook_signing_key) | Per-rack HMAC secret that signs outbound webhook deliveries with a Convox-Signature header. |
