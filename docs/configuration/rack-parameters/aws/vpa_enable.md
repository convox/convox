---
title: "vpa_enable"
slug: vpa_enable
url: /configuration/rack-parameters/aws/vpa_enable
---

# vpa_enable

## Description
The `vpa_enable` parameter installs the Vertical Pod Autoscaler (VPA) on the rack. VPA automatically adjusts CPU and memory requests for services based on observed usage patterns, right-sizing each replica's resource allocation over time.

## Default Value
The default value for `vpa_enable` is `false`.

## Use Cases
- **Resource Optimization**: Automatically right-size service resource requests based on actual usage instead of manual estimation.
- **Cost Reduction**: Avoid over-provisioning CPU and memory by letting VPA recommend or apply optimal values.
- **Performance Improvement**: Prevent under-provisioning that can lead to CPU throttling or OOM kills.

## Setting Parameters
To enable VPA on your rack, use the following command:
```bash
$ convox rack params set vpa_enable=true -r rackName
Setting parameters... OK
```

## Additional Information
Enabling VPA installs the VPA controller, recommender, and admission controller in the cluster. Once enabled, services can use the `scale.vpa` section in their `convox.yml` to configure vertical autoscaling.

See [Scaling](/deployment/scaling#vertical-pod-autoscaler-vpa) for service configuration details.
