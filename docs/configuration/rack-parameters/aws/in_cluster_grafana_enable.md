---
title: "in_cluster_grafana_enable"
slug: in_cluster_grafana_enable
url: /configuration/rack-parameters/aws/in_cluster_grafana_enable
---

# in_cluster_grafana_enable

## Description
Enable the bundled Grafana sub-chart inside the paid `kube-prometheus-stack` Helm release. When set to `true`, a Grafana pod runs inside the rack alongside the Convox-managed Prometheus, with sidecar discovery enabled for ConfigMaps carrying the `grafana_dashboard=1` label. The in-cluster Grafana auto-imports the GPU observability dashboards Convox ships at `terraform/cluster/aws/dashboards/`.

Defaults to `false` because most users either run their own external Grafana (Grafana Cloud, self-hosted) or deploy the standalone Grafana convox app from `convox-examples/grafana-gpu-dashboards`. Enabling adds a single Grafana pod to the rack, which is convenient but is no substitute for a production-managed Grafana with persistent storage, SSO, alerting, and centralized dashboards.

The bundled Grafana ships ephemerally — its SQLite database lives in the pod's emptyDir, so user-saved tweaks are lost on pod restart. The dashboards re-provision idempotently from ConfigMap discovery, so the dashboard set itself is durable. For persistent state, use one of the alternatives in the Use Cases below.

## Default Value
The default value for `in_cluster_grafana_enable` is `false`.

## Use Cases
- **Quick-start GPU observability**: Enable for a turnkey in-cluster Grafana that auto-imports the Convox-authored GPU dashboards (cluster overview, per-app GPU, inference performance, GPU health, vLLM, Karpenter node lifecycle). Useful for early evaluation or single-rack environments.
- **Demo / staging racks**: Bundle Grafana into the rack to give engineers a reachable URL without standing up a separate observability stack.
- **Air-gapped clusters**: Where Grafana Cloud is unreachable and standing up a separate Grafana app is overkill, the bundled in-cluster Grafana provides observability without leaving the cluster.

## Setting Parameters
To enable the bundled Grafana, also set an admin password (defense-in-depth — the chart refuses to render Grafana with an unset admin password):
```bash
$ convox rack params set in_cluster_grafana_enable=true in_cluster_grafana_admin_password=<your-strong-password> -r rackName
Setting parameters... OK
```

To disable:
```bash
$ convox rack params set in_cluster_grafana_enable=false -r rackName
Setting parameters... OK
```

Disabling removes the Grafana pod and its Service. The dashboard ConfigMaps remain (they are deployed by the rack's TF independently) — they go inert until another Grafana with sidecar discovery is configured to consume them.

## Additional Information
- This parameter is AWS-only at this time. GCP, Azure, DigitalOcean, and Equinix Metal racks ship parallel Grafana integrations in subsequent releases.
- `in_cluster_grafana_enable=true` requires `in_cluster_grafana_admin_password` to be set. The chart will not render an unauthenticated Grafana.
- The bundled Grafana SQLite database is **ephemeral** — pod restart resets user-saved tweaks (custom panels, alert rules, API keys, anonymous-viewer counters). The Convox-authored dashboards re-import on every pod start, so the dashboard set itself is durable.
- For persistent state, one of:
  - Add a `database:` Postgres resource to your convox stack and configure Grafana with `GF_DATABASE_TYPE=postgres`
  - Run a separate Grafana app (e.g., `convox-examples/grafana-gpu-dashboards` with a Postgres backend)
  - Connect external Grafana Cloud / self-hosted to the rack's Prometheus
- The bundled Grafana sidecar discovers ConfigMaps with `grafana_dashboard=1` label cluster-wide. The Convox-authored dashboards live at `terraform/cluster/aws/dashboards/` (deployed by the rack TF when `gpu_observability_enable=true`) and auto-import on Grafana startup.

## Related Parameters
- [in_cluster_grafana_admin_password](/configuration/rack-parameters/aws/in_cluster_grafana_admin_password): Required when this parameter is `true`. Sets the Grafana admin user's password.
- [grafana_url](/configuration/rack-parameters/aws/grafana_url): Set to the in-cluster Grafana's service URL so the Console GPU view's deep-link button targets the bundled Grafana.
- [gpu_observability_enable](/configuration/rack-parameters/aws/gpu_observability_enable): Installs the DCGM exporter that emits the GPU metrics the bundled Grafana dashboards consume.
- [prometheus_url](/configuration/rack-parameters/aws/prometheus_url): The query-side Prometheus endpoint the rack uses for `convox ps` GPU enrichment. Independent of the in-cluster Grafana.

## Version Requirements
This parameter requires at least Convox rack version `3.24.6`.
