# examples/gpu-llm — GPU observability SSOT

Source-of-truth for Convox's GPU observability dashboards.

## What's here

- `grafana/*.json` — 6 Grafana dashboards (D1 cluster, D2 per-app, D3 inference, D5 health, D6 vLLM, D7 Karpenter). Authored once here; consumed by:
  - Rack-side TF: `terraform/cluster/aws/dashboards/` (synced via `make sync-dashboards`)
  - Standalone Grafana convox app: `convox-examples/grafana-gpu-dashboards/` (org repo; dashboards committed inline)

- `grafana/promql-source-of-truth.yaml` — YAML mirror of `provider/k8s/prometheus_queries.go` const declarations. Bidirectional parity test at `provider/k8s/prometheus_parity_test.go` enforces no drift.

- `grafana/LICENSE-vllm.txt` — Apache 2.0 attestation for D6 (adapted from grafana.com dashboard 23991, vLLM upstream).

## Editing dashboards

1. Edit `grafana/<dashboard>.json` directly.
2. If a panel target uses a new PromQL string:
   - Add a corresponding const to `provider/k8s/prometheus_queries.go`
   - Mirror the new const into `grafana/promql-source-of-truth.yaml`
   - Annotate the panel target with `_source: "<CONST_NAME>"`
3. For upstream-derived JSONs (e.g. D6 vLLM), tag the dashboard root with `"convox_source_check": "upstream"` so forward-parity skips it. Use `_source: "upstream:<name>"` per panel target. Convox-authored extensions to upstream JSONs use `_source: "convox-authored:<name>"`.
4. Run `make sync-dashboards` to copy to TF dir.
5. Run `make verify-dashboards-synced` to validate parity.
6. Run `go test ./provider/k8s/ -run TestPromQL -count=1` to verify Go ↔ YAML ↔ JSON parity.

## What lives elsewhere

For user-facing examples (vLLM convox app, standalone Grafana app), see the `convox-examples` GitHub org:
- `convox-examples/llm-gpu-api` — FastAPI + GPU autoscaling
- `convox-examples/llama2-convox-chatbot` — vLLM + Llama 2 + React frontend
- `convox-examples/grafana-gpu-dashboards` — standalone Grafana convox app pre-loaded with the dashboards above
