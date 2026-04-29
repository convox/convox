---
title: "gpu_observability_chart_version"
slug: gpu_observability_chart_version
url: /configuration/rack-parameters/aws/gpu_observability_chart_version
---

# gpu_observability_chart_version

## Description
The `gpu_observability_chart_version` parameter pins the Helm chart version for the NVIDIA DCGM exporter installed by [`gpu_observability_enable`](/configuration/rack-parameters/aws/gpu_observability_enable). Change this value to roll forward to a CVE hotfix or driver-compatibility release without waiting for a Convox rack version that bumps the default.

## Default Value
The default value is the chart version that ships with this rack release — currently `4.8.1` (image tag `4.5.2-4.8.1-distroless`).

## Use Cases
- **CVE response**: When NVIDIA publishes a security fix in a new chart patch release (e.g., `4.8.1` → `4.8.2`), pin to the patched version immediately rather than waiting for the next Convox rack release.
- **Driver compatibility**: If your AMI ships a specific NVIDIA driver version that requires a particular DCGM exporter version for full metric coverage, pin to the matching chart.
- **Rollback after a regression**: If a chart patch introduces a regression on your workload, pin back to the prior known-good version while you investigate.

## Setting Parameters
To pin to a specific chart version:
```bash
$ convox rack params set gpu_observability_chart_version=4.8.2 -r rackName
Setting parameters... OK
```

To revert to the rack default:
```bash
$ convox rack params set gpu_observability_chart_version=4.8.1 -r rackName
Setting parameters... OK
```

You must enable [`gpu_observability_enable`](/configuration/rack-parameters/aws/gpu_observability_enable) for the chart to be installed at all — pinning the version while observability is disabled is a no-op until you enable it.

## Additional Information
- Stay on the same chart major version (e.g., within `4.x`) when pinning. Chart majors may introduce CRDs or admission webhooks that the Convox installer does not yet handle, which can break clean uninstall on downgrade.
- The chart audited for `4.8.1` (the default at this rack release) installs zero CRDs and zero admission webhooks, so `helm uninstall` cleanly removes all resources. A future chart major (e.g., `5.x`) requires a re-audit and possibly a Convox rack release before adoption is safe — see the chart-bump checklist Convox maintains for the rack provider.
- Always verify the chart you pin to is published at the NVIDIA upstream Helm repo: `https://nvidia.github.io/dcgm-exporter/helm-charts`. Convox does not vendor the chart.

## Related Parameters
- [gpu_observability_enable](/configuration/rack-parameters/aws/gpu_observability_enable): The enable switch that controls whether the chart is installed at all.
- [nvidia_device_plugin_enable](/configuration/rack-parameters/aws/nvidia_device_plugin_enable): Required prerequisite for `gpu_observability_enable=true`.

## Version Requirements
This feature requires at least Convox rack version `3.24.6`.
