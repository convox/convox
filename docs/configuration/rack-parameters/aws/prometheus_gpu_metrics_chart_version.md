---
title: "prometheus_gpu_metrics_chart_version"
slug: prometheus_gpu_metrics_chart_version
url: /configuration/rack-parameters/aws/prometheus_gpu_metrics_chart_version
---

# prometheus_gpu_metrics_chart_version

## Description
The `prometheus_gpu_metrics_chart_version` parameter pins the Helm chart version for the free-plan Prometheus chart deployed via the Convox Console (`prometheus-community/prometheus`). Applies on next Disable→Enable cycle from the Convox Console. Change this value to roll forward to a CVE hotfix release without waiting for a Convox rack version that bumps the default.

The chart is `prometheus-community/prometheus` from the upstream `https://prometheus-community.github.io/helm-charts` repo. The release is named `prometheus-gpu-metrics` and lives in the `kube-system` namespace.

## Default Value
The default value is the chart version that ships with this rack release — currently `27.9.0`.

## Use Cases
- **CVE response**: When the upstream chart publishes a security fix patch (e.g., `27.9.0` → `27.9.1`), pin to the patched version immediately rather than waiting for the next Convox rack release.
- **Rollback after an issue**: If a chart patch introduces an issue on your workload, pin back to the prior known-good version while you investigate.

## Setting Parameters
To pin to a specific chart version:
```bash
$ convox rack params set prometheus_gpu_metrics_chart_version=27.9.1 -r rackName
Setting parameters... OK
```

To revert to the rack default:
```bash
$ convox rack params set prometheus_gpu_metrics_chart_version=27.9.0 -r rackName
Setting parameters... OK
```

You must enable monitoring (free plan) in the Convox Console for the free-plan Prometheus chart to be deployed at all — pinning the version is otherwise a no-op until you enable monitoring. The new chart version takes effect on the next Disable→Enable cycle from the Console.

## Additional Information
- Stay on the same chart major version (e.g., within `27.x`) when pinning. Chart majors may introduce subchart name changes or values-schema breakage that the Convox provider does not yet handle.
- The chart audited for `27.9.0` (the default at this rack release) installs zero CRDs and zero admission webhooks, so `helm uninstall` cleanly removes all resources. A future chart major requires a re-audit and possibly a Convox rack release before adoption is safe.
- Always verify the chart you pin to is published at the prometheus-community upstream Helm repo: `https://prometheus-community.github.io/helm-charts`. Convox does not vendor the chart.

## Related Parameters
- [gpu_observability_enable](/configuration/rack-parameters/aws/gpu_observability_enable): Enables the DCGM exporter on this rack. The Console-deployed Prometheus chart scrapes DCGM when monitoring is enabled in the Console.
- [prometheus_gpu_metrics_retention](/configuration/rack-parameters/aws/prometheus_gpu_metrics_retention): Retention window for the free-plan Prometheus chart deployed via the Convox Console.

## Version Requirements
This parameter requires at least Convox rack version `3.24.6`.
