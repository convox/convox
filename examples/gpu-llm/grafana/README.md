# Grafana JSON dashboards

Six GPU observability dashboards as raw Grafana JSON. Hand-authored once,
delivered through three surfaces:

- ConfigMap auto-install via the rack TF (`terraform/cluster/aws/dashboards.tf`)
- Direct JSON import (this directory)
- Standalone Grafana convox app (`convox-examples/grafana-gpu-dashboards`)

The source-of-truth manifest is `promql-source-of-truth.yaml`.

## Dashboards

| File | Title | Audience | Source |
|------|-------|----------|--------|
| `01-cluster-overview.json` | GPU Cluster Overview | rack operator | Convox-authored |
| `02-per-app-gpu.json` | Per-App GPU Usage | app developer | Convox-authored |
| `03-inference-performance.json` | Inference Workload Performance | app developer | Convox-authored |
| `05-gpu-health.json` | GPU Health & Errors | rack operator | Convox-authored |
| `06-vllm.json` | LLM Inference Server (vLLM) | ML engineer | Adapted from upstream — Apache 2.0 |
| `07-karpenter.json` | GPU Node Lifecycle (Karpenter) | platform engineer | Convox-authored |

A GPU cost overlay dashboard is on the roadmap for a future release.

## Manual import (BYO Grafana)

```text
In Grafana UI:
  1. + → Import → Upload JSON file
  2. Select 01-cluster-overview.json (and the others)
  3. Map your Prometheus datasource when prompted
```

Each dashboard exposes the standard Grafana template variables:
- `$datasource` — Prometheus DS UID
- `$cluster` — multi-cluster (label `rack`)
- `$namespace` — convox app's K8s namespace (e.g. `myrack-myapp`)
- `$service` — convox service inside the app
- `$gpu_uuid` — DCGM `UUID` for per-GPU drilldowns
- `$node` — K8s node name (DCGM `Hostname` or kubelet `node`)

## Console GPU view deep-link

Configure your Grafana base URL in Rack Settings; the "Open in your Grafana"
button on Console GPU views will deep-link there. The button is inert
until you've imported these JSONs into your Grafana — it constructs a URL of
the form `<grafana-url>/d/<dashboard-slug>?var-namespace=<app-ns>` which 404s if
the dashboard isn't present.

The Console manages your in-cluster Prometheus URL automatically when monitoring
is enabled. For users running their own external Prometheus, set
`prometheus_url` on the rack manually.

## License (Dashboard 6 — vLLM)

`06-vllm.json` is adapted from the
[vLLM project](https://github.com/vllm-project/vllm) under Apache License 2.0.
The attestation is in `LICENSE-vllm.txt`. Convox-authored extensions to the
vLLM dashboard (KV-cache pressure current and trend panels) are under the
parent repository's license.

## Editing dashboards

If you need to change a panel's PromQL:

1. Edit the source-of-truth Go const at `provider/k8s/prometheus_queries.go`.
2. Mirror the change in `promql-source-of-truth.yaml`.
3. Update the corresponding `expr` in the JSON file (keep the
   `"_source": "<CONST_NAME>"` annotation).
4. Run `make sync-dashboards` from the repo root to refresh the rack TF copy.
5. The CI parity tests
   (`go test ./provider/k8s -run 'TestPromQL|TestEveryPanel'`) catch drift.

Vue panels in the Console don't consume raw PromQL — they consume
scalar/timeseries numbers via GraphQL resolvers. The rack PromQL methods are
the only consumer of the Go consts at runtime.

## Maintenance triggers

- **Karpenter chart-version bump** in `terraform/cluster/aws/karpenter.tf`:
  re-verify `karpenter_*` metric set hasn't drifted (panel JSON expressions may
  need updating). The scrape config in `pkg/structs/karpenter-scrape.yaml` uses
  port number 8080 (more stable across versions than port name).
- **dcgm-exporter chart-version bump** in `terraform/cluster/aws/dcgm.tf`:
  re-verify `terraform/cluster/aws/files/dcp-metrics-included.csv` against the
  new chart's `default-counters.csv` baseline; the override append-set may need
  pruning if the new chart already exposes a previously-missing field.
- **Grafana version bump** for the standalone app or in-cluster sidecar:
  re-run the `_source` annotation round-trip survival check. Annotation
  strip-on-save would invalidate the parity test.
