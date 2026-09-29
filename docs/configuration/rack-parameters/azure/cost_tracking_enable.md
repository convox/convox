---
title: "cost_tracking_enable"
description: "The cost_tracking_enable Azure rack parameter turns on the rack-side cost accumulator that powers convox cost and per-app budget caps, defaulting to false."
slug: cost_tracking_enable
url: /configuration/rack-parameters/azure/cost_tracking_enable
---

# cost_tracking_enable

## Description
The `cost_tracking_enable` parameter turns on the Rack's cost accumulator, which estimates each App's VM compute spend and powers [`convox cost`](/reference/cli/cost) and per-App [budget caps](/management/budget-caps). The accumulator runs inside the Rack API. On each tick, every 10 minutes, it charges each running pod the share of its node's hourly price that the pod's CPU or memory requests reserve, whichever is larger, or its share of the node's GPUs when it requests GPUs. The result is added to the App's spend for the current calendar month, which resets on the 1st, UTC. Spend is stored as an annotation on the App's namespace and surfaces in `convox cost`, the Convox Console cost views, and budget enforcement.

On Azure, spend is priced against eastus Linux list prices, keyed by the VM size in the node's `node.kubernetes.io/instance-type` label (for example `Standard_D4s_v5`). Spot node pools are detected from the `kubernetes.azure.com/priority` or `kubernetes.azure.com/scalesetpriority` node label and discounted automatically.

Cost tracking is a prerequisite for budget caps. While it is off, the Rack rejects `convox budget cap raise`, and any `convox budget set` that sets a monthly cap, alert threshold or at-cap action, with HTTP 422. It also rejects every promote of an App whose `convox.yml` `budget:` block sets `monthlyCapUsd`, `alertThresholdPercent` or `atCapAction`. The error points at this parameter.

## Default Value
The default value for `cost_tracking_enable` is `false`. Users must opt in to enable the cost accumulator and budget-cap surfaces.

## Use Cases
- **Per-App cost visibility**: See each App's estimated month-to-date spend by Service in `convox cost` and the Convox Console, without an external cost-management tool.
- **Budget caps**: Set a monthly cap with `convox budget set <app> --monthly-cap 1000` and have the Rack alert, block new deploys, or scale Services to zero when the App's month-to-date spend reaches it.

## Capacity Considerations
The accumulator runs inside the Rack API and adds no pods to the cluster. The Console cost views read the Rack's cost data directly and do not depend on Console monitoring or Prometheus.

Console monitoring is a separate Console setting that installs a Prometheus chart. The paid-plan kube-prometheus-stack chart runs in the `convox-monitoring` namespace with a steady-state footprint of roughly 1 vCPU and 2 GiB of memory, and install-time spikes can reach 1.5x that. On a Rack with a single small workload node it can overcommit the node and leave pods stuck in Terminating. Before you enable Console monitoring on such a Rack, do one of the following:
- Run one workload node of `Standard_D2s_v5` or larger (or any 2 vCPU / 4+ GiB size).
- Run two or more workload nodes of any size, so user pods can spread off the Prometheus node.
- Leave autoscaling headroom on the default node pool ([max_on_demand_count](/configuration/rack-parameters/azure/max_on_demand_count) above current usage) so the cluster can grow to absorb the install spike.

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
- Cost tracking is available on AWS from Rack version `3.24.6`, on Azure from `3.25.1` and on GCP from `3.25.9`. See [cost_tracking_enable (AWS)](/configuration/rack-parameters/aws/cost_tracking_enable) and [cost_tracking_enable (GCP)](/configuration/rack-parameters/gcp/cost_tracking_enable) for the other providers.
- The accumulator runs inside the Rack API, not as a sidecar. It ticks once when it starts and then every 10 minutes, and no parameter changes the interval.
- The pricing table ships with the Rack version and cannot be edited. Azure prices are eastus Linux consumption list prices. `convox cost --aggregate` shows the table date: `pricing-table:2026-09-28` from `3.25.9`.
- Estimates use list prices, so discounts Azure applies to your bill, such as reservations or savings plans, are not reflected, and neither are other regions' rates. The pricing adjustment on an App's budget scales its estimate by a multiplier from `0.1` to `1.5`: `convox budget set my-app --monthly-cap 1000 --pricing-adjustment 0.7` records 70% of the estimate.
- A pod on an AKS spot node pool is priced at `0.30` of its VM size's pay-as-you-go rate, except on 19 GPU sizes that carry their own spot factor in the table. A node with no capacity label at all is priced at the pay-as-you-go rate and shows capacity `unknown`.
- A VM size missing from the table adds no spend, and its pods are counted in `warning-count`. See [Unpriced instance types](/management/cost-tracking#unpriced-instance-types).
- Spend covers the current calendar month and resets on the 1st, UTC. The Rack keeps no earlier months.

## Related Parameters
- [webhook_signing_key](/configuration/rack-parameters/azure/webhook_signing_key): Webhook deliveries from cost-tracking events (`app:budget:auto-shutdown:armed`, `app:budget:auto-shutdown:fired`) carry an HMAC signature when this is set, so receivers can verify authenticity.

## Version Requirements
This parameter requires at least Convox rack version `3.25.1`.
