---
title: "prometheus_url"
slug: prometheus_url
url: /configuration/rack-parameters/aws/prometheus_url
---

# prometheus_url

## Description
The `prometheus_url` parameter overrides the rack's auto-resolved Prometheus endpoint with a user-supplied URL. The rack-API queries this endpoint on each `convox ps` / `convox services` invocation to enrich responses with `gpu-util`, `gpu-mem-used`, `gpu-mem-total` per-pod fields.

**In default cases, no configuration is required.** The rack auto-resolves an in-cluster Prometheus endpoint in priority order (highest first):
1. **Convox metered metrics enabled** (paid path): a `kube-prometheus-stack` Prometheus is installed (release `convox` in `convox-monitoring` ns). The rack queries `http://convox-kube-prometheus-sta-prometheus.convox-monitoring.svc.cluster.local:9090`.
2. **`gpu_observability_enable=true`** (free path): the rack installs a lightweight Prometheus scoped to scraping DCGM metrics (release `prometheus-gpu-metrics` in `kube-system` ns). The rack queries `http://prometheus-gpu-metrics-server.kube-system.svc.cluster.local:80`. Suppressed when paid metered is enabled.

Set this parameter ONLY when you have an external Prometheus aggregator (Grafana Cloud Prometheus, AWS AMP, or a self-hosted federation hub) and want the rack to query that aggregator INSTEAD. **The user-set value always wins** over the auto-resolution chain.

The parameter value is treated as a credential and is stored only in a Kubernetes Secret on the rack — never plaintext, never in deploy-spec annotations. Telemetry emits a SHA-256 hash, never the URL.

## Default Value
The default is `""` (empty string). Empty value triggers the auto-resolution above. **For most users, leave this empty** — the rack handles configuration automatically based on whether you have GPU observability or metered metrics enabled. If neither is enabled and `prometheus_url` is empty, GPU metric enrichment is silently skipped (no error; `convox ps` returns responses without GPU fields).

## Use Cases
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

To revert to the rack default (in-cluster Prometheus):
```bash
$ convox rack params set prometheus_url='' -r rackName
Updating parameters... OK
```

## Additional Information
- This parameter is AWS-only at this time. GCP, Azure, DigitalOcean, and Equinix Metal racks ship parallel Prometheus integrations in subsequent releases.
- The value is treated as sensitive: stored as a Kubernetes Secret (not a ConfigMap), never logged in plaintext, never serialized into rack deploy-spec annotations, and SHA-256-hashed before emission to telemetry.
- The rack's HTTP client uses a 5-second timeout per query so a misconfigured or unreachable endpoint cannot stall `convox ps` indefinitely. On query timeout the rack returns the response without GPU enrichment fields populated; the UI displays an em-dash sentinel rather than an error.
- The rack queries the Prometheus standard `/api/v1/query` endpoint with the DCGM-style metric labels (`DCGM_FI_DEV_GPU_UTIL`, `DCGM_FI_DEV_FB_USED`, `DCGM_FI_DEV_FB_TOTAL`). If your external Prometheus does not have those metric series, GPU enrichment falls through to the empty state.
- The auto-resolution priority is: user-set `prometheus_url` > paid metered metrics Prometheus > free-tier GPU Prometheus (`gpu_observability_enable`) > none. Whether the DCGM exporter runs inside the rack is controlled by `gpu_observability_enable`; this parameter only overrides the QUERY-side endpoint.

## Related Parameters
- [gpu_observability_enable](/configuration/rack-parameters/aws/gpu_observability_enable): Installs the DCGM exporter that emits the metrics this parameter queries. The two work together but each can be set independently.
- [gpu_observability_chart_version](/configuration/rack-parameters/aws/gpu_observability_chart_version): Pin the DCGM exporter chart version when needed.

## Version Requirements
This parameter requires at least Convox rack version `3.24.6`.
