---
title: "Scaling"
slug: scaling
url: /configuration/scaling
---
# Scaling

Convox provides several approaches to scaling your services, from simple manual adjustments to fully automated event-driven scaling.

## Autoscaling

Configure horizontal scaling with preconfigured triggers or manual replica counts.

The simplest path to autoscaling is the `scale.autoscale` block, which provides preconfigured KEDA-based triggers for CPU, memory, GPU utilization, and queue depth with just a threshold value:

```yaml
services:
  web:
    scale:
      min: 2
      max: 10
      autoscale:
        cpu:
          threshold: 70
```

This scales the service between 2 and 10 replicas, targeting 70% CPU utilization. Scale-to-zero is supported with `min: 0`. See [Autoscaling](/configuration/scaling/autoscaling) for the full trigger reference and examples.

> `scale.autoscale` requires `keda_enable=true` on the rack (AWS only). CPU and memory autoscaling via `scale.targets` works on all providers without KEDA.

## Vertical Pod Autoscaler (VPA)

Automatically right-size CPU and memory requests for your services based on observed usage. VPA adjusts resource allocation per replica rather than changing the number of replicas.

See [VPA](/configuration/scaling/vpa) for details.
> AWS only

## KEDA Autoscaling (Advanced)

For event sources beyond the four built-in `scale.autoscale` triggers (SQS queue depth, cron schedules, Datadog queries, CloudWatch metrics, and 60+ others), use `scale.keda.triggers` with raw KEDA trigger configuration. Supports scale-to-zero.

See [KEDA Autoscaling](/configuration/scaling/keda) for details.
> AWS only

## Datadog Metrics Autoscaling

Scale services based on business-level metrics from Datadog via HPA external metrics. Useful when scaling decisions depend on request rates, queue depths, or other application-specific signals. If you use KEDA, you can also scale on Datadog metrics via the KEDA Datadog scaler. See [KEDA Autoscaling](/configuration/scaling/keda#keda-with-datadog-metrics).

See [Datadog Metrics Autoscaling](/configuration/scaling/datadog-metrics) for details.
> All providers (requires Datadog Cluster Agent)

## Karpenter

Opt-in alternative to Cluster Autoscaler for AWS EKS node provisioning. Karpenter provisions the optimal instance type and size in seconds rather than minutes, supports scale-to-zero builds, automatic node consolidation, and cost-aware instance selection across spot and on-demand capacity.

See [Karpenter](/configuration/scaling/karpenter) for details.
> AWS only

## Workload Placement

Control which nodes your services run on using custom node groups, node selectors, and dedicated node pools. Use this to isolate workloads, target GPU nodes, or optimize cost by routing services to specific instance types.

See [Workload Placement](/configuration/scaling/workload-placement) for details.
> AWS and Azure

## Console Autoscale Triggers

Enable, disable, and tune autoscaling from the Console without editing convox.yml. Console-driven triggers override the manifest configuration and persist across deploys.

See [Autoscale Triggers](/console/autoscale-triggers) for details.
