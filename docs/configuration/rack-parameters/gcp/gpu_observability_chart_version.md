---
title: "gpu_observability_chart_version"
description: "The gpu_observability_chart_version GCP rack parameter pins the Helm chart version for the NVIDIA DCGM exporter installed by gpu_observability_enable."
slug: gpu_observability_chart_version
url: /configuration/rack-parameters/gcp/gpu_observability_chart_version
---

# gpu_observability_chart_version

## Description
The `gpu_observability_chart_version` parameter pins the Helm chart version for the NVIDIA DCGM exporter installed by [`gpu_observability_enable`](/configuration/rack-parameters/gcp/gpu_observability_enable). Change this value to roll forward to a CVE hotfix or driver-compatibility release without waiting for a Convox rack version that bumps the default.

## Default Value
The default value is the chart version that ships with this rack release, currently `4.8.1`.

## Use Cases
- **CVE response**: When NVIDIA publishes a security fix in a new chart patch release, pin to the patched version immediately rather than waiting for the next Convox rack release.
- **Driver compatibility**: Pin to a chart version that matches the GKE-managed NVIDIA driver for full metric coverage.
- **Rollback after an issue**: Pin back to the prior known-good version while you investigate a regression.

## Setting Parameters
To pin to a specific chart version:
```bash
$ convox rack params set gpu_observability_chart_version=4.8.2 -r rackName
Updating parameters... OK
```

To revert to the rack default:
```bash
$ convox rack params set gpu_observability_chart_version=4.8.1 -r rackName
Updating parameters... OK
```

You must enable [`gpu_observability_enable`](/configuration/rack-parameters/gcp/gpu_observability_enable) for the chart to be installed. Pinning the version while observability is disabled is a no-op until you enable it.

## Additional Information
- Stay on the same chart major version (e.g., within `4.x`) when pinning. A new chart major may introduce custom resources or admission webhooks that the rack does not yet handle, which can break clean uninstall.
- Always verify the chart you pin to is published at the NVIDIA upstream Helm repo: `https://nvidia.github.io/dcgm-exporter/helm-charts`. Convox does not vendor the chart.

## Related Parameters
- [gpu_observability_enable](/configuration/rack-parameters/gcp/gpu_observability_enable): The enable switch that controls whether the chart is installed at all.
- [dcgm_scrape_interval](/configuration/rack-parameters/gcp/dcgm_scrape_interval): Scrape interval hint set as a pod annotation on the DCGM exporter.

## Version Requirements
This feature requires at least Convox rack version `3.25.2`.
