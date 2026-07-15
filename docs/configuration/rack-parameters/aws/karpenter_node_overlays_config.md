---
title: "karpenter_node_overlays_config"
description: "The karpenter_node_overlays_config AWS rack parameter creates Karpenter NodeOverlays to adjust scheduling capacity and pricing for selected instance types."
slug: karpenter_node_overlays_config
url: /configuration/rack-parameters/aws/karpenter_node_overlays_config
---

# karpenter_node_overlays_config

## Description

The `karpenter_node_overlays_config` parameter creates [Karpenter](/configuration/scaling/karpenter) NodeOverlays. A NodeOverlay adjusts how Karpenter simulates a matching set of instance types: it can advertise extended resources (for example `nvidia.com/gpu`), override the price Karpenter uses in its cost model, or apply a percentage price adjustment.

Setting a non-empty value also enables the Karpenter `NodeOverlay` feature gate on the controller. An empty value leaves the gate off and creates no NodeOverlay resources.

The primary use is fractional-GPU instance families such as `g6f`/`gr6f`, which AWS reports with a GPU count of zero. Karpenter will price those types but will not provision them for `nvidia.com/gpu` requests until an overlay advertises the resource. Overlays are generic, so the same parameter also covers Savings-Plan-aware pricing and instance-family cost penalties.

## Default Value

The default value is empty (feature gate off, no NodeOverlays).

## Setting the Parameter

**Using a JSON string:**

```bash
$ convox rack params set karpenter_node_overlays_config='[{"name":"g6f-fractional-gpu","weight":100,"requirements":[{"key":"node.kubernetes.io/instance-type","operator":"In","values":["g6f.large","g6f.xlarge","g6f.2xlarge","g6f.4xlarge","gr6f.4xlarge"]}],"capacity":{"nvidia.com/gpu":"1"}}]' -r rackName
Updating parameters... OK
```

**Using a JSON file:**

```bash
$ convox rack params set karpenter_node_overlays_config=/path/to/overlays.json -r rackName
Updating parameters... OK
```

**Clearing the parameter:**

```bash
$ convox rack params set karpenter_node_overlays_config= -r rackName
Updating parameters... OK
```

## Entry Fields

| Field | Required | Description |
|-------|----------|-------------|
| `name` | yes | Lowercase alphanumeric with dashes, max 63 chars. Unique across entries. |
| `requirements` | yes | Non-empty array of `{key, operator, values}` selecting the instance types the overlay applies to. `operator` is one of `In`, `NotIn`, `Exists`, `DoesNotExist`, `Gt`, `Lt`. `values` is required except for `Exists`/`DoesNotExist`; `Gt`/`Lt` take exactly one value. |
| `capacity` | no | Extended resources to advertise, for example `{"nvidia.com/gpu":"1"}`. Standard resources (`cpu`, `memory`, `ephemeral-storage`, `pods`) are not allowed. |
| `price` | no | Absolute hourly price used in Karpenter's cost model. Mutually exclusive with `priceAdjustment`. |
| `priceAdjustment` | no | Signed adjustment to the computed price, either absolute (`+5`, `-1.5`) or percentage (`-30%`, capped at `-100%`). Mutually exclusive with `price`. |
| `weight` | no | Integer 1-10000. Higher weight wins when multiple overlays match the same instance type. |

Each entry must set at least one of `capacity`, `price`, or `priceAdjustment`.

## Additional Information

- **Input formats:** Raw JSON string, base64-encoded JSON, or a `.json` file path.
- Requires `karpenter_enabled=true` to take effect. The value is accepted and stored on any rack, but NodeOverlays are only created once Karpenter is enabled.
- `capacity` affects scheduling simulation only; the node must actually provide the resource. For GPUs this means the [NVIDIA device plugin](/configuration/rack-parameters/aws/nvidia_device_plugin_enable) must be running so the advertised `nvidia.com/gpu` is real.
- Overlay changes can take a few minutes to affect provisioning decisions, and enabling the first overlay restarts the Karpenter controller (a brief provisioning pause).
- NodeOverlay is an alpha upstream API (`karpenter.sh/v1alpha1`). Revalidate overlay configurations when the Karpenter chart is upgraded.

## See Also

- [Karpenter](/configuration/scaling/karpenter#node-overlays-fractional-gpus) for the NodeOverlay section and the fractional-GPU (g6f) walkthrough
- [additional_karpenter_nodepools_config](/configuration/rack-parameters/aws/additional_karpenter_nodepools_config) for custom NodePools
- [karpenter_config](/configuration/rack-parameters/aws/karpenter_config) for overriding the workload NodePool
