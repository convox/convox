---
title: "prometheus_url"
slug: prometheus_url
url: /configuration/rack-parameters/aws/prometheus_url
---

# prometheus_url

## Description
External Prometheus URL for KEDA autoscale triggers and observability. Setting this value enables GPU metrics in `convox ps`. When empty (default), GPU fields display as a dash (`-`) even when a Prometheus chart is installed via Console.

The rack queries this URL directly; it is not auto-resolved. Set the in-cluster service URL when you enable Convox Console monitoring, or point at an external Prometheus aggregator (Grafana Cloud, AWS AMP, federation hub).

The parameter value is treated as a credential. It is stored as a Kubernetes Secret on the rack, never in plaintext, and never logged. Convox telemetry emits only a hash of the value, never the URL itself.

## Default Value
The default is `""` (empty string). When empty, GPU metrics in `convox ps` are silently skipped: there is no error, and the GPU fields render as a dash (`-`) instead of a value.

## Use Cases
- **Convox-Console-managed users**: must explicitly set this to surface GPU fields in `convox ps`. Use the in-cluster service URL: paid → `http://convox-kube-prometheus-sta-prometheus.convox-monitoring.svc.cluster.local:9090`, free → `http://prometheus-gpu-metrics-server.kube-system.svc.cluster.local:80`.
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
Note: clearing `prometheus_url` removes KEDA autoscale based on Prometheus metrics and disables `convox ps` GPU metrics until the parameter is re-set. The rack does not auto-resolve a URL; empty means empty.

## Security: address validation

To prevent the rack from being pointed at a sensitive internal endpoint, the following address types are rejected:

- Private CIDRs (`10/8`, `172.16/12`, `192.168/16`)
- Loopback (`127/8`)
- Link-local (`169.254/16`)
- Unspecified (`0.0.0.0`)
- DNS hostnames that resolve into any of the ranges above

Valid endpoints include public hostnames (Grafana Cloud, AWS AMP, your aggregator) and in-cluster service hostnames matching `*.svc.cluster.local`. The in-cluster pattern is explicitly allowed so the in-cluster Prometheus URLs in the "Use Cases" section work.

If your value is rejected, set the in-cluster service URL or a public hostname instead. The same check runs when the rack starts up, so a value that was applied another way (for example, by editing rack config directly) is rejected at startup and GPU metrics stay disabled until it is corrected.

## Additional Information
- This parameter is AWS-only at this time.
- The value is treated as sensitive: stored as a Kubernetes Secret (not a ConfigMap), never logged in plaintext, and hashed before emission to telemetry.
- The rack uses a 5-second timeout per query so a misconfigured or unreachable endpoint cannot stall `convox ps` indefinitely. On a query timeout, the response is returned without GPU fields populated and the UI shows a dash (`-`) rather than an error.
- The rack queries the standard Prometheus `/api/v1/query` endpoint for four DCGM metric series: `DCGM_FI_DEV_GPU_UTIL`, `DCGM_FI_DEV_FB_USED`, `DCGM_FI_DEV_FB_FREE`, and `DCGM_FI_DEV_FB_RESERVED`. Total framebuffer is derived as the sum of the three `FB_*` fields. If your external Prometheus does not have these metric series, GPU fields stay empty.
- Whether the DCGM exporter runs inside the rack is controlled by `gpu_observability_enable`; this parameter only configures the query endpoint. Convox Console deploys the Prometheus chart that scrapes DCGM independently of this parameter. Set `prometheus_url` to that chart's in-cluster service URL to wire `convox ps` GPU metrics to it.

## Related Parameters
- [gpu_observability_enable](/configuration/rack-parameters/aws/gpu_observability_enable): Installs the DCGM exporter that emits the metrics this parameter queries. The two work together but each can be set independently.
- [gpu_observability_chart_version](/configuration/rack-parameters/aws/gpu_observability_chart_version): Pin the DCGM exporter chart version when needed.

## Version Requirements
This parameter requires at least Convox rack version `3.24.6`.
