---
title: "prometheus_url"
slug: prometheus_url
url: /configuration/rack-parameters/aws/prometheus_url
---

# prometheus_url

## Description
The `prometheus_url` parameter points the rack at a Prometheus-compatible metrics endpoint that the api-pod queries when populating GPU-utilization and GPU-memory fields on the rack-API surface (`convox ps -j`, `convox services -j`). With this parameter set, the rack queries the configured endpoint on each `convox ps` / `convox services` invocation and enriches the response with `gpu-util`, `gpu-mem-used`, and `gpu-mem-total` per-pod fields and `gpu-util-avg`, `gpu-mem-used-avg`, and `gpu-mem-total-avg` per-service averages.

When unset (the default), the rack falls back to its in-cluster Prometheus that runs alongside KEDA. For most rack operators this default is sufficient — the in-cluster Prometheus already scrapes the DCGM exporter installed by `gpu_observability_enable`. Set this parameter when you have an external Prometheus aggregator (Grafana Cloud Prometheus, AWS AMP, or a self-hosted federation hub) and want the rack to query that aggregator instead.

The parameter value is treated as a credential and is stored only in a Kubernetes Secret on the rack — never in the plaintext ConfigMap and never in the rack's deploy-spec annotations. Rack telemetry (heartbeat to metrics.convox.com) emits a SHA-256 hash of the value, never the plaintext URL.

## Default Value
The default value for `prometheus_url` is `""` (empty string). Empty value means "use the in-cluster Prometheus" — no external endpoint is queried.

## Use Cases
- **Grafana Cloud federation**: Point the rack at your Grafana Cloud Prometheus URL (with HTTP Basic auth credentials embedded in the URL) so GPU metrics surface in Convox dashboards while your central observability stack also retains them.
- **AWS Managed Prometheus (AMP)**: Direct rack queries to your AMP workspace for centralized rentention and longer query windows than the in-cluster Prometheus offers.
- **Self-hosted federation hub**: When you run a multi-cluster Prometheus federation, point the rack at the federated query endpoint for cross-rack metric aggregation.
- **External DCGM stack**: If you maintain DCGM exporters outside Convox (e.g., on shared GPU nodes outside the rack's control), point the rack at the Prometheus that scrapes them so Convox surfaces remain consistent.

## Setting Parameters
To set a custom Prometheus endpoint:
```bash
$ convox rack params set prometheus_url=https://prometheus.example.com -r rackName
Updating parameters... OK
```

If your Prometheus uses HTTP Basic auth in URL form:
```bash
$ convox rack params set prometheus_url='https://user:token@prom.example.com' -r rackName
Updating parameters... OK
```

To revert to the rack default (in-cluster Prometheus):
```bash
$ convox rack params set prometheus_url='' -r rackName
Updating parameters... OK
```

## Additional Information
- This parameter is AWS-only at this time. GCP, Azure, DigitalOcean, and Equinix Metal racks ship parallel Prometheus integrations in subsequent releases.
- The value is treated as sensitive: stored as a Kubernetes Secret (not a ConfigMap), never logged in plaintext, never serialized into rack deploy-spec annotations, and SHA-256-hashed before emission to telemetry.
- The rack's HTTP client uses a 5-second timeout per query so a misconfigured or unreachable endpoint cannot stall `convox ps` indefinitely. On query timeout the rack returns the response without GPU enrichment fields populated; the customer sees an em-dash sentinel in the UI rather than an error.
- The rack queries the Prometheus standard `/api/v1/query` endpoint with the DCGM-style metric labels (`DCGM_FI_DEV_GPU_UTIL`, `DCGM_FI_DEV_FB_USED`, `DCGM_FI_DEV_FB_TOTAL`). If your external Prometheus does not have those metric series, GPU enrichment falls through to the empty state.
- Pairs with `gpu_observability_enable` for the in-cluster scrape path. This parameter ONLY affects the QUERY side; whether the DCGM exporter runs inside the rack is controlled by `gpu_observability_enable`.

## Related Parameters
- [gpu_observability_enable](/configuration/rack-parameters/aws/gpu_observability_enable): Installs the DCGM exporter that emits the metrics this parameter queries. The two work together but each can be set independently.
- [gpu_observability_chart_version](/configuration/rack-parameters/aws/gpu_observability_chart_version): Pin the DCGM exporter chart version when needed.

## Version Requirements
This parameter requires at least Convox rack version `3.24.6`.
