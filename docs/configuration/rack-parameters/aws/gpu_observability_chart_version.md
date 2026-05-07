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
- The chart audited for `4.8.1` (the default at this rack release) installs zero CRDs and zero admission webhooks, so `helm uninstall` cleanly removes all resources. A future chart major (e.g., `5.x`) requires a re-audit and possibly a Convox rack release before adoption is safe — see the [Chart-bump checklist](#chart-bump-checklist) below.
- Always verify the chart you pin to is published at the NVIDIA upstream Helm repo: `https://nvidia.github.io/dcgm-exporter/helm-charts`. Convox does not vendor the chart.

## Chart-bump Checklist
When bumping the DCGM exporter chart **major** version (e.g., `4.x` → `5.x`), the Convox provider must re-audit the chart before merging the bump. The default chart version on a rack release is set by the Convox rack provider; out-of-band patches via this parameter stay within the same major. Anything that changes the cleanup contract requires provider work, not a user-side pin.

Audit the new chart major against these four areas:

- **CRD audit**: Inspect `helm template <new-chart-version>` for any `kind: CustomResourceDefinition` outputs. The `4.x` default installs zero CRDs. If a new major adds CRDs, the Convox provider must add a finalizer-cleanup `null_resource` mirroring the [karpenter.tf:172-242 pattern](https://github.com/convox/convox/blob/main/terraform/cluster/aws/karpenter.tf) so `helm uninstall` does not orphan in-cluster resources on disable.
- **Webhook audit**: Inspect for `kind: ValidatingWebhookConfiguration` or `kind: MutatingWebhookConfiguration` outputs. The `4.x` default installs zero admission webhooks. A webhook on a new major typically requires a cert-manager dependency or a chart-managed self-signing path; the Convox provider must verify the webhook does not block in-cluster traffic during upgrade and that the webhook's owning Deployment cleans up on `helm uninstall`.
- **Finalizer behavior**: Inspect Deployments, DaemonSets, and ServiceAccounts in the chart for `metadata.finalizers` blocks. The `4.x` default uses Kubernetes-default finalizer behavior (no chart-managed finalizers). A new major that adds finalizers can deadlock `terraform destroy` on the `helm_release.dcgm_exporter` resource — match the karpenter pattern with a pre-uninstall `null_resource` that strips finalizers before the helm release is destroyed.
- **Kubelet pod-resources socket compatibility**: The DCGM exporter relies on the NVIDIA device plugin's `/var/lib/kubelet/pod-resources/` socket for pod-to-GPU attribution. New chart majors may bump the minimum kubelet API version or change the socket-discovery path. Verify against the EKS-supported Kubernetes versions Convox racks use (currently `1.28`, `1.29`, `1.30`) before bumping.
- **`podAnnotations` rendering contract**: The Convox installer relies on a chart values key that renders into the DaemonSet pod template's `spec.template.metadata.annotations`. The `convox.com/dcgm-csv-<hash>` annotation in [`terraform/cluster/aws/dcgm.tf`](https://github.com/convox/convox/blob/main/terraform/cluster/aws/dcgm.tf) (currently `-sha256`; [`provider/k8s/dcgm_csv_test.go`'s `TestDcgmExporterCSV_RolloutAnnotationPresent`](https://github.com/convox/convox/blob/main/provider/k8s/dcgm_csv_test.go) accepts any `convox.com/dcgm-csv-*` suffix bound to any of `filesha256`, `filesha1`, or `filemd5`) is what forces a DaemonSet roll on `dcp-metrics-included.csv` content changes — without that roll, customers stay on the old CSV after rack upgrades and have to manually `kubectl rollout restart daemonset/dcgm-exporter`. Verify the new chart's contract with:

  ```sh
  helm template dcgm-exporter <chart-repo>/dcgm-exporter \
    --version <new-chart-version> \
    --set podAnnotations.convox-test=value | \
    yq 'select(.kind=="DaemonSet") | .spec.template.metadata.annotations'
  ```

  Confirm `convox-test: value` appears in the output. (Targeting the DaemonSet by `.kind` avoids confusion with the chart's other resources — ConfigMap, Service, ServiceAccount — which also have `metadata.annotations` fields irrelevant to this contract.) If the chart no longer honors top-level `podAnnotations`, identify the new sub-key (`daemonset.podAnnotations`, `dcgmExporter.podAnnotations`, `extraPodAnnotations`, `pod.annotations`, or whatever the chart now uses), update the helm values block in [`dcgm.tf`](https://github.com/convox/convox/blob/main/terraform/cluster/aws/dcgm.tf), AND update `TestDcgmExporterCSV_RolloutAnnotationPresent`'s `extractBracedBlock(src, regexp.MustCompile(...))` opening pattern to match the new key. A chart that silently nests podAnnotations under an unaudited key disables the auto-heal mechanism without surfacing a TF/helm error.

Once the audit completes cleanly and any required provider-side mitigations land, bump the default chart version in [`terraform/cluster/aws/variables.tf`](https://github.com/convox/convox/blob/main/terraform/cluster/aws/variables.tf) and [`terraform/system/aws/variables.tf`](https://github.com/convox/convox/blob/main/terraform/system/aws/variables.tf) as part of the next Convox rack release.

## Related Parameters
- [gpu_observability_enable](/configuration/rack-parameters/aws/gpu_observability_enable): The enable switch that controls whether the chart is installed at all.
- [nvidia_device_plugin_enable](/configuration/rack-parameters/aws/nvidia_device_plugin_enable): Required prerequisite for `gpu_observability_enable=true`.

## Version Requirements
This feature requires at least Convox rack version `3.24.6`.
