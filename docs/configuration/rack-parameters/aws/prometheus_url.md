---
title: "prometheus_url"
slug: prometheus_url
url: /configuration/rack-parameters/aws/prometheus_url
---

# prometheus_url

## Description
External Prometheus URL for KEDA autoscale triggers and observability. Customer-set value enables GPU enrichment in `convox ps`. When empty (default), GPU fields show em-dash sentinels even when a chart is installed via Console.

Post-3.24.6 there is no auto-resolution — the rack queries this URL directly. Set the in-cluster service URL when you enable Convox Console monitoring, or point at an external Prometheus aggregator (Grafana Cloud, AWS AMP, federation hub).

The parameter value is treated as a credential and is stored only in a Kubernetes Secret on the rack — never plaintext, never in deploy-spec annotations. Telemetry emits a SHA-256 hash, never the URL.

## Default Value
The default is `""` (empty string). When empty, GPU metric enrichment in `convox ps` is silently skipped (no error; `convox ps` returns responses without GPU fields populated, rendering as em-dash sentinels).

## Use Cases
- **Convox-Console-managed customers**: must explicitly set this to surface GPU fields in `convox ps`. Use the in-cluster service URL: paid → `http://convox-kube-prometheus-sta-prometheus.convox-monitoring.svc.cluster.local:9090`, free → `http://prometheus-gpu-metrics-server.kube-system.svc.cluster.local:80`.
- **Grafana Cloud federation**: Point the rack at your Grafana Cloud Prometheus URL (with HTTP Basic auth credentials embedded in the URL) so GPU metrics surface in Convox dashboards while your central observability stack also retains them.
- **AWS Managed Prometheus (AMP)**: Direct rack queries to your AMP workspace for centralized retention and longer query windows than the in-cluster Prometheus offers.
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

To clear the parameter (e.g. switching observability stacks):
```bash
$ convox rack params set prometheus_url='' -r rackName
Updating parameters... OK
```
Note: clearing `prometheus_url` removes KEDA autoscale based on Prometheus metrics and disables `convox ps` GPU enrichment until the parameter is re-set. There is no rack-side auto-resolution post-3.24.6 — empty means empty.

## Additional Information
- This parameter is AWS-only at this time. GCP, Azure, DigitalOcean, and Equinix Metal racks ship parallel Prometheus integrations in subsequent releases.
- The value is treated as sensitive: stored as a Kubernetes Secret (not a ConfigMap), never logged in plaintext, never serialized into rack deploy-spec annotations, and SHA-256-hashed before emission to telemetry.
- The rack's HTTP client uses a 5-second timeout per query so a misconfigured or unreachable endpoint cannot stall `convox ps` indefinitely. On query timeout the rack returns the response without GPU enrichment fields populated; the UI displays an em-dash sentinel rather than an error.
- The rack queries the Prometheus standard `/api/v1/query` endpoint with the DCGM-style metric labels (`DCGM_FI_DEV_GPU_UTIL`, `DCGM_FI_DEV_FB_USED`, `DCGM_FI_DEV_FB_TOTAL`). If your external Prometheus does not have those metric series, GPU enrichment falls through to the empty state.
- Whether the DCGM exporter runs inside the rack is controlled by `gpu_observability_enable`; this parameter only configures the QUERY-side endpoint. The Convox Console deploys the Prometheus chart that scrapes DCGM independently of this parameter — set `prometheus_url` to that chart's in-cluster service URL to wire `convox ps` GPU enrichment to it.

## Related Parameters
- [gpu_observability_enable](/configuration/rack-parameters/aws/gpu_observability_enable): Installs the DCGM exporter that emits the metrics this parameter queries. The two work together but each can be set independently.
- [gpu_observability_chart_version](/configuration/rack-parameters/aws/gpu_observability_chart_version): Pin the DCGM exporter chart version when needed.

## Version Requirements
This parameter requires at least Convox rack version `3.24.6`.
