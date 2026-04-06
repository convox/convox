---
title: "Scaling"
slug: scaling
url: /configuration/scaling
---
# Scaling

Convox provides several approaches to scaling your services, from simple manual adjustments to fully automated event-driven scaling.

## Autoscaling

Configure horizontal scaling based on CPU and memory utilization, set initial resource defaults, manually adjust replica counts, and allocate GPUs for accelerated workloads. This is the starting point for most scaling needs.

See [Autoscaling](/configuration/scaling/autoscaling) for details.

## Vertical Pod Autoscaler (VPA)

Automatically right-size CPU and memory requests for your services based on observed usage. VPA adjusts resource allocation per replica rather than changing the number of replicas.

See [VPA](/configuration/scaling/vpa) for details.
> AWS only

## KEDA Autoscaling

Event-driven autoscaling powered by KEDA. Scale from external signals like SQS queue depth, cron schedules, Datadog queries, CloudWatch metrics, or any of KEDA's 60+ supported scalers. Supports scale-to-zero for cost optimization.

See [KEDA Autoscaling](/configuration/scaling/keda) for details.
> AWS only

## Datadog Metrics Autoscaling

Scale services based on business-level metrics from Datadog via HPA external metrics. Useful when scaling decisions depend on request rates, queue depths, or other application-specific signals. If you use KEDA, you can also scale on Datadog metrics via the KEDA Datadog scaler -- see [KEDA Autoscaling](/configuration/scaling/keda#keda-with-datadog-metrics).

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
