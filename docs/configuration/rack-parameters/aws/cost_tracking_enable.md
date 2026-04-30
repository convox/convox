---
title: "cost_tracking_enable"
slug: cost_tracking_enable
url: /configuration/rack-parameters/aws/cost_tracking_enable
---

# cost_tracking_enable

## Description
The `cost_tracking_enable` parameter turns on the rack-side cost-accumulator that powers `convox cost` and per-app budget caps. With cost tracking enabled, the rack samples the resource usage of every running pod (CPU millicores, memory megabytes, GPU count, GPU vendor) on a fixed cadence, multiplies each sample by the per-instance pricing table from `pkg/billing/prices.go`, and persists the rolling-window total as a Kubernetes namespace annotation on each app. The cost data is exposed through the `AppCost` API and surfaces in Console3 cost dashboards, the `convox cost` CLI, and the budget-cap auto-shutdown machinery.

Cost tracking is a prerequisite for the per-app budget cap feature (`convox budget set`). With cost tracking disabled, budget caps cannot fire because the accumulator is not running — `convox budget set` is rejected at the CLI with a friendly error pointing here.

## Default Value
The default value for `cost_tracking_enable` is `false`. Customers must opt in to enable the cost accumulator and budget-cap surfaces.

## Use Cases
- **Per-app cost visibility**: Surface dollars-per-hour by app and by service in Console3 dashboards and the `convox cost` CLI without integrating an external cost-management tool.
- **Budget-cap enforcement**: Set a monthly USD cap on an app via `convox budget set --monthly-cap-usd 1000 <app>` and have the rack auto-block deploys (or auto-shutdown services) when the rolling 30-day spend reaches the cap.
- **Cost-per-utilization analysis**: Combined with `gpu_observability_enable`, surface dollars-per-actual-GPU-hour rather than dollars-per-allocated-GPU-hour for inference and training workloads.
- **Spend forecasting**: The accumulator emits a rolling spend rate that feeds Console3 forecasting widgets so customers can see "at this burn rate, you'll hit your cap in 6 days."

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

Disabling drops the cost-accumulator goroutine on the next api-pod restart. Existing `convox.com/budget-state` and `convox.com/budget-shutdown-state` annotations are left intact (no data destruction); they simply stop receiving new samples until the parameter is re-enabled.

## Additional Information
- This parameter is currently AWS-only. GCP, Azure, DigitalOcean, and Equinix Metal racks ship parallel cost-tracking backends in subsequent releases.
- The cost accumulator runs in the api-pod, not as a sidecar — sampling cadence is bounded by the api-pod's existing resource budget. There is no additional CPU or memory allocation introduced by enabling this parameter.
- The pricing table at `pkg/billing/prices.go` is updated per Convox release. Pin a custom multiplier (e.g., to model your AWS Enterprise Discount Program) via the per-app `pricing-adjustment` budget option (`convox budget set --pricing-adjustment 0.7 <app>`).
- Spot capacity-type discount is applied automatically (`SpotDefaultFactor = 0.30`) when the underlying NodePool is configured for spot. Per-instance overrides are captured via the `SpotUsdPerHourFactor` field in the pricing table.
- The rolling-window length is 30 days. Older samples roll off the tail as new samples arrive; the rack does not retain unbounded historical data.

## Related Parameters
- [gpu_observability_enable](/configuration/rack-parameters/aws/gpu_observability_enable): Pairs with cost tracking to surface dollars-per-actual-GPU-utilization metrics.
- [webhook_signing_key](/configuration/rack-parameters/aws/webhook_signing_key): Webhook deliveries from cost-tracking events (`app:budget:armed`, `app:budget:fired`) carry an HMAC signature when this is set, so receivers can verify authenticity.

## Version Requirements
This parameter requires at least Convox rack version `3.24.6`.
