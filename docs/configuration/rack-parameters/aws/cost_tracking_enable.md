---
title: "cost_tracking_enable"
description: "The cost_tracking_enable AWS rack parameter turns on the rack-side cost accumulator that powers convox cost and per-app budget caps, defaulting to false."
slug: cost_tracking_enable
url: /configuration/rack-parameters/aws/cost_tracking_enable
---

# cost_tracking_enable

## Description
The `cost_tracking_enable` parameter turns on the rack-side cost-accumulator that powers `convox cost` and per-app budget caps. With cost tracking enabled, the rack samples the resource usage of every running pod (CPU millicores, memory megabytes, GPU count, GPU vendor) on a fixed cadence, multiplies each sample by the rack's per-instance pricing table, and persists the rolling-window total as a Kubernetes namespace annotation on each app. The cost data is exposed through the `AppCost` API and surfaces in the Convox Console cost dashboards, the `convox cost` CLI, and the budget-cap auto-shutdown machinery.

Cost tracking is a prerequisite for the per-app budget cap feature (`convox budget set`). With cost tracking disabled, budget caps cannot fire because the accumulator is not running. `convox budget set` is rejected at the CLI with a friendly error pointing here.

## Default Value
The default value for `cost_tracking_enable` is `false`. Users must opt in to enable the cost accumulator and budget-cap surfaces.

## Use Cases
- **Per-app cost visibility**: Surface dollars-per-hour by app and by service in the Convox Console dashboards and the `convox cost` CLI without integrating an external cost-management tool.
- **Budget-cap enforcement**: Set a monthly USD cap on an app via `convox budget set --monthly-cap 1000 <app>` and have the rack auto-block deploys (or auto-shutdown services) when the rolling 30-day spend reaches the cap.
- **Cost-per-utilization analysis**: Combined with `gpu_observability_enable`, surface dollars-per-actual-GPU-hour rather than dollars-per-allocated-GPU-hour for inference and training workloads.
- **Spend forecasting**: The accumulator emits a rolling spend rate that feeds Convox Console forecasting widgets so users can see "at this burn rate, you'll hit your cap in 6 days."

## Capacity Considerations
The rack-side cost accumulator itself runs inside the existing rack control plane and adds no separately-allocated CPU or memory on the cluster. However, surfacing cost data in the Convox Console additionally installs the kube-prometheus-stack helm chart (Prometheus operator + state-metrics + a single-replica Prometheus statefulset + node-exporter daemonset) into the `convox-monitoring` namespace when the user enables monitoring through the Console UI. The chart's combined steady-state footprint is roughly 1 vCPU and 2 GiB of memory; transient install-time spikes can be 1.5x that.

For racks with a single small workload node (e.g. `t3.small`, `t3.medium`), enabling Console monitoring on top of cost tracking can overcommit the node and trigger a kubelet failure that drags pods into a stuck Terminating state. Recommended minimums for clusters that intend to surface cost data through the Console:
- One workload node of `t3.large` or larger (or any 2 vCPU / 4+ GiB instance), OR
- Two or more workload nodes of any size where the user-workload pods can spread off the prometheus statefulset's node, OR
- Karpenter enabled on the rack (`karpenter_enable=true`) so the rack can grow capacity on demand.

Convox 3.24.6 ships explicit resource requests on the prometheus statefulset and a PodDisruptionBudget so Karpenter pre-provisions a fitting node before scheduling and so voluntary disruption pauses while a replacement reschedules. Smaller clusters still benefit from enabling Karpenter so the rack can react to chart-install spikes.

## Setting Parameters
To enable cost tracking on an existing rack:
```bash
$ convox rack params set cost_tracking_enable=true -r rackName
Updating parameters... OK
```

To disable:
```bash
$ convox rack params set cost_tracking_enable=false -r rackName
Updating parameters... OK
```

Disabling stops the cost accumulator on the next rack restart. Existing budget-state annotations on each app are left intact (no data destruction); they stop receiving new samples until the parameter is re-enabled.

## Additional Information
- This parameter is currently AWS-only.
- The cost accumulator runs inside the rack control plane, not as a sidecar. Sampling cadence is bounded by the rack's existing resource budget. There is no additional CPU or memory allocation introduced by enabling this parameter.
- The pricing table is updated per Convox release. Pin a custom multiplier (e.g., to model your AWS Enterprise Discount Program) via the per-app `pricing-adjustment` budget option (`convox budget set --pricing-adjustment 0.7 <app>`).
- Spot capacity-type discount is applied automatically (a default factor of `0.30`) when the underlying NodePool is configured for spot. Per-instance overrides are configurable via the rack's pricing table.
- The rolling-window length is 30 days. Older samples roll off the tail as new samples arrive; the rack does not retain unbounded historical data.

## Related Parameters
- [gpu_observability_enable](/configuration/rack-parameters/aws/gpu_observability_enable): Pairs with cost tracking to surface dollars-per-actual-GPU-utilization metrics.
- [webhook_signing_key](/configuration/rack-parameters/aws/webhook_signing_key): Webhook deliveries from cost-tracking events (`app:budget:armed`, `app:budget:fired`) carry an HMAC signature when this is set, so receivers can verify authenticity.

## Version Requirements
This parameter requires at least Convox rack version `3.24.6`.
