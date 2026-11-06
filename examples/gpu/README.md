# GPU Smoke Test Example

This minimal example deploys the NVIDIA CUDA base image and runs `nvidia-smi`
to verify GPU availability on a Convox rack with GPU support enabled.

## Requirements

- A Convox V3 rack on AWS with GPU nodes (e.g. `node_type=g4dn.xlarge`).
- `gpu_observability_enable=true` set on the rack to install the DCGM exporter.

## Usage

```bash
convox deploy
convox logs
```

The `nvidia-smi` output confirms the GPU is visible to the container runtime.

## Prometheus scrape annotations

To expose metrics from an inference server (vLLM, Triton, SGLang) for the GPU
observability dashboard, add scrape annotations to the service in `convox.yml`:

```yaml
services:
  inference:
    # ... your image/command/scale config ...
    annotations:
      prometheus.io/scrape: "true"
      prometheus.io/port: "8000"
      prometheus.io/path: "/metrics"
```

The Convox GPU observability dashboards (dashboards 01-07, enabled via
`gpu_observability_enable=true` + monitoring enabled in Console) will pick up
per-pod metrics from the scrape endpoint automatically.

## See Also

- [GPU Observability](/configuration/gpu-observability) — rack-level setup
- [GPU scaling](/configuration/scaling/gpu) — `scale.gpu` parameter reference
