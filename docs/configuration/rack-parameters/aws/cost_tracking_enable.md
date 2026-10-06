---
title: "cost_tracking_enable"
description: "The cost_tracking_enable AWS rack parameter turns on the rack-side cost accumulator that powers convox cost and per-app budget caps, defaulting to false."
slug: cost_tracking_enable
url: /configuration/rack-parameters/aws/cost_tracking_enable
---

# cost_tracking_enable

## Description
The `cost_tracking_enable` parameter turns on the Rack's cost accumulator, which estimates each App's EC2 compute spend and powers [`convox cost`](/reference/cli/cost) and per-App [budget caps](/management/budget-caps). The accumulator runs inside the Rack API. On each tick, every 10 minutes, it charges each running pod the share of its node's hourly price that the pod's CPU or memory requests reserve, whichever is larger, or its share of the node's GPUs when it requests GPUs. The result is added to the App's spend for the current calendar month, which resets on the 1st, UTC. From Rack version `3.25.10` the Rack also keeps each UTC day's spend for [`cost_tracking_history_days`](/configuration/rack-parameters/aws/cost_tracking_history_days) days, for date-range queries. Spend is stored as an annotation on the App's namespace and surfaces in `convox cost`, the Convox Console cost views, and budget enforcement.

On AWS, spend is priced against us-east-1 Linux on-demand list prices, keyed by the node's `node.kubernetes.io/instance-type` label (for example `m5.large`).

Cost tracking is a prerequisite for budget caps. While it is off, the Rack rejects `convox budget cap raise`, and any `convox budget set` that sets a monthly cap, alert threshold or at-cap action, with HTTP 422. It also rejects every promote of an App whose `convox.yml` `budget:` block sets `monthlyCapUsd`, `alertThresholdPercent` or `atCapAction`. The error points at this parameter.

## Default Value
The default value for `cost_tracking_enable` is `false`. Users must opt in to enable the cost accumulator and budget-cap surfaces.

## Use Cases
- **Per-App cost visibility**: See each App's estimated month-to-date spend by Service in `convox cost` and the Convox Console, without an external cost-management tool.
- **Budget caps**: Set a monthly cap with `convox budget set <app> --monthly-cap 1000` and have the Rack alert, block new deploys, or scale Services to zero when the App's month-to-date spend reaches it.
- **GPU spend by Service**: A pod that requests GPUs is charged for its share of the node's GPUs, so each GPU Service's cost appears in its own row.

## Capacity Considerations
The accumulator runs inside the Rack API and adds no pods to the cluster. The Console cost views read the Rack's cost data directly and do not depend on Console monitoring or Prometheus.

Console monitoring is a separate Console setting that installs a Prometheus chart. The capacity that chart needs on a Rack with small workload nodes is covered under [gpu_observability_enable](/configuration/rack-parameters/aws/gpu_observability_enable).

## Setting Parameters
To enable cost tracking on an existing rack:
```bash
$ convox rack params set cost_tracking_enable=true -r rackName
Updating parameters... OK
```

The change updates the Rack API Deployment and creates no cloud resources. An App's spend first appears on the second tick after the update completes, about 10 minutes later.

To disable:
```bash
$ convox rack params set cost_tracking_enable=false -r rackName
Updating parameters... OK
```

The accumulator stops once the update has rolled the Rack API. Stored spend is kept, and `convox cost` shows the last snapshot with a notice that tracking is disabled. Deploy blocking and auto-shutdown stop with it, and Services that auto-shutdown already scaled to zero stay at zero until `convox budget reset`. A promote of an App whose `budget:` block sets `monthlyCapUsd`, `alertThresholdPercent` or `atCapAction` is rejected until the block is removed or tracking is enabled again, including the re-promote that `convox apps params set`, `convox apps lock` and `convox apps unlock` run.

## Additional Information
- Cost tracking is available on AWS from Rack version `3.24.6`, on Azure from `3.25.1` and on GCP from `3.25.9`. See [cost_tracking_enable (Azure)](/configuration/rack-parameters/azure/cost_tracking_enable) and [cost_tracking_enable (GCP)](/configuration/rack-parameters/gcp/cost_tracking_enable) for provider details.
- The accumulator runs inside the Rack API, not as a sidecar. It ticks once when it starts and then every 10 minutes, and no parameter changes the interval.
- The pricing table ships with the Rack version and cannot be edited. `convox cost --aggregate` shows its date: `pricing-table:2026-09-28` from `3.25.9`.
- Estimates use list prices, so discounts AWS applies to your bill, such as Savings Plans, Reserved Instances or an Enterprise Discount Program, are not reflected. The pricing adjustment on an App's budget scales its estimate by a multiplier from `0.1` to `1.5`: `convox budget set my-app --monthly-cap 1000 --pricing-adjustment 0.7` records 70% of the estimate.
- A node labeled `karpenter.sh/capacity-type=spot` (Karpenter) or `eks.amazonaws.com/capacityType=SPOT` (an EKS spot node group) is priced at `0.30` of its instance type's on-demand rate. No AWS entry in the table has its own spot factor, so every AWS spot row uses that default. A node with no capacity label at all is priced on-demand and shows capacity `unknown`.
- An instance type missing from the table adds no spend, and its pods are counted in `warning-count`. See [Unpriced instance types](/management/cost-tracking#unpriced-instance-types).
- Month-to-date spend covers the current calendar month and resets on the 1st, UTC. Racks before `3.25.10` keep no earlier spend; from `3.25.10` the daily history above is kept separately and is not reset.

## Related Parameters
- [cost_tracking_history_days](/configuration/rack-parameters/aws/cost_tracking_history_days): Days of daily App cost history kept for `convox cost --start` and `--end` and the Console date ranges.
- [gpu_observability_enable](/configuration/rack-parameters/aws/gpu_observability_enable): Exports DCGM GPU utilization metrics. Cost tracking does not read them: it charges a GPU pod for the GPUs it requests, whether it uses them or not.
- [webhook_signing_key](/configuration/rack-parameters/aws/webhook_signing_key): Webhook deliveries from cost-tracking events (`app:budget:auto-shutdown:armed`, `app:budget:auto-shutdown:fired`) carry an HMAC signature when this is set, so receivers can verify authenticity.

## Version Requirements
This parameter requires at least Convox rack version `3.24.6`.
