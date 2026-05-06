---
title: "grafana_url"
slug: grafana_url
url: /configuration/rack-parameters/aws/grafana_url
---

# grafana_url

## Description
External Grafana base URL surfaced as the "Open in your Grafana" deep-link button on the Convox Console GPU views. Lets users running their own Grafana (Grafana Cloud, self-hosted, in-cluster sidecar of the paid kube-prometheus-stack chart) jump from a Convox-rendered GPU panel directly to the matching dashboard with rack/app/service template variables prefilled.

When empty (default), the deep-link button hides itself entirely from the Console GPU views — there is no inert state. Users without Grafana see no extra UI affordance, by design. Customers who later set the value see the button appear on next page load.

The button constructs URLs of the form `<grafana_url>/d/<dashboard-uid>/?var-rack=<rack>&var-namespace=<rack>-<app>&var-service=<svc>&from=now-1h&to=now`. The Convox-authored dashboards (D1 cluster, D2 per-app, D3 inference, D5 health, D6 vLLM, D7 Karpenter) ship with stable UIDs (`convox-gpu-cluster-overview`, `convox-gpu-per-app`, etc.). The deep link 404s in your Grafana until you import the dashboards from `examples/gpu-llm/grafana/*.json` or deploy the standalone Grafana convox app from `convox-examples/grafana-gpu-dashboards`.

## Default Value
The default is `""` (empty string). When empty, the Console GPU view's "Open in your Grafana" button is hidden; no error, no inert link.

## Use Cases
- **BYO Grafana Cloud**: Set `grafana_url=https://yourorg.grafana.net` so the Console deep-links into your team's existing Grafana Cloud workspace alongside your other observability dashboards.
- **Self-hosted Grafana**: Set to the URL of your in-VPC Grafana so engineers landing on a Convox GPU panel can drill into your full Grafana with the right template variables prefilled.
- **Standalone Convox-deployed Grafana**: Deploy `convox-examples/grafana-gpu-dashboards` into the same rack and set this parameter to the deployed app's domain so the button takes you to the bundled Grafana with the dashboards pre-imported.

## Setting Parameters
To wire the deep-link button to a Grafana instance:
```bash
$ convox rack params set grafana_url=https://grafana.example.com -r rackName
Updating parameters... OK
```

To clear the parameter (button disappears from the Console GPU views):
```bash
$ convox rack params set grafana_url='' -r rackName
Updating parameters... OK
```

## Additional Information
- This parameter is AWS-only at this time. GCP, Azure, DigitalOcean, and Equinix Metal racks ship parallel Grafana deep-link integrations in subsequent releases.
- Setting `grafana_url` has no effect on rack workloads or the Convox-managed Prometheus chart. It is purely a Console-side URL hint — the rack itself never reaches out to the configured URL.
- The button is hidden until set, so there is no broken-link UX before the dashboards are imported. Standard import path: `git clone https://github.com/convox/convox && cd convox/examples/gpu-llm/grafana && # import the *.json files into Grafana`.
- Trailing slashes in the URL are stripped client-side before constructing the deep link, so `https://grafana.example.com`, `https://grafana.example.com/`, and `https://grafana.example.com//` all resolve identically.

## Related Parameters
- [enable_in_cluster_grafana](/configuration/rack-parameters/aws/enable_in_cluster_grafana): Enables the bundled Grafana sub-chart in the paid kube-prometheus-stack chart. When enabled, set `grafana_url` to the in-cluster Grafana service URL so the deep-link button targets the bundled Grafana.
- [in_cluster_grafana_admin_password](/configuration/rack-parameters/aws/in_cluster_grafana_admin_password): Admin password for the in-cluster Grafana when `enable_in_cluster_grafana=true`.
- [prometheus_url](/configuration/rack-parameters/aws/prometheus_url): The query-side Prometheus endpoint the rack uses for `convox ps` GPU enrichment. Independent of `grafana_url`; both can be set.
- [gpu_observability_enable](/configuration/rack-parameters/aws/gpu_observability_enable): Installs the DCGM exporter that emits the metrics the Grafana dashboards consume.

## Version Requirements
This parameter requires at least Convox rack version `3.24.6`.
