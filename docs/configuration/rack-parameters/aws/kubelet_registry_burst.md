---
title: "kubelet_registry_burst"
description: "The kubelet_registry_burst AWS rack parameter sets the maximum image pull requests allowed in a burst beyond registry_pull_qps, defaulting to 10."
slug: kubelet_registry_burst
url: /configuration/rack-parameters/aws/kubelet_registry_burst
---

# kubelet_registry_burst

## Description

The `kubelet_registry_burst` parameter defines the maximum number of image pull requests that can be made in a burst, exceeding the `registry_pull_qps` limit for a short duration. This parameter allows for short-lived spikes in image pull traffic.

The limiter does not queue. A pull that arrives once the burst allowance is spent fails immediately and enters a backoff of roughly 10 to 300 seconds, which the pod reports as `ErrImagePull`. Raising this value raises the number of concurrent cold-start pulls that succeed rather than queueing the rest.

## Default Value

The default value for `kubelet_registry_burst` is `10`.

This parameter works together with [kubelet_registry_pull_qps](/configuration/rack-parameters/aws/kubelet_registry_pull_qps), which controls the steady-state rate limit (queries per second) that the burst rate is permitted to exceed for a short duration.

## Use Cases

- **Handle burst traffic**: Allow for temporary spikes in image pull requests without exceeding the registry_pull_qps limit.
- **Improve pod startup time**: Permit a higher initial burst of image pulls to accelerate pod startup.
- **High Deployment Frequency**: Increase limits in environments with frequent container deployments.
- **Large Container Images**: Optimize pull rates for environments with large image sizes.
- **Registry Rate Limiting**: Adjust limits to prevent hitting registry-imposed rate limits.
- **Cluster Scale-Up**: Improve node startup time by allowing faster concurrent image pulls.
- **CI/CD Optimization**: Accelerate deployments in continuous integration/deployment pipelines.

## Setting Parameters

**Changing the value replaces every node on the Rack,** so schedule the change.

To enable the `kubelet_registry_burst` parameter, use the following command:
```bash
$ convox rack params set kubelet_registry_burst=value -r rackName
Updating parameters... OK
```

Replace value with the desired maximum number of burst image pull requests.

## Additional Information

This parameter is available on AWS Racks only and requires Rack version `3.25.6` or later.

- **Accepted values:** integers, with a minimum effective value of `1`. Nothing is rejected, and a value outside the legal range is moved to the nearest legal one: a value below `1` becomes `1`, and anything above `2147483647` is capped there. Fractions are truncated, so a value between the default and the next integer up truncates back to the default and the node sees no change.
- **A value of `0` does not mean no limit.** A zero burst would admit no pull at all, so the Rack raises it to `1`. This is the reverse of [`kubelet_registry_pull_qps`](/configuration/rack-parameters/aws/kubelet_registry_pull_qps), where `0` turns the limiter off.
- The `kubelet_registry_burst` parameter complements `kubelet_registry_pull_qps` by providing flexibility in handling short-lived spikes in image pull traffic. However, excessive burst values can still overload the registry. It's essential to consider the average image pull rate and the expected peak load when setting this value.
- **Node scope:** the parameter reaches the system node group, the build node group, additional node groups, additional build node groups, and the Karpenter workload, build, and additional NodePools. It does not reach a Karpenter workload pool running Bottlerocket ([`karpenter_node_os`](/configuration/rack-parameters/aws/karpenter_node_os) set to `bottlerocket`), which takes its own kubelet settings. Setting either `karpenter_config.ec2NodeClass.userData` or `karpenter_config.ec2NodeClass.amiSelectorTerms` suppresses the parameter on the Karpenter workload pool, and either one does so on its own; the build and additional Karpenter pools are unaffected. See [`karpenter_config`](/configuration/rack-parameters/aws/karpenter_config).
- **Node replacement pacing:** managed node groups recycle one node at a time unless [`node_max_unavailable_percentage`](/configuration/rack-parameters/aws/node_max_unavailable_percentage) is set, which paces the system node group and additional node groups; the build node groups have no pacing setting and always recycle one node at a time. Karpenter pools are paced by their disruption budgets. On a large Rack, set `node_max_unavailable_percentage` before changing this parameter.
- To verify the setting on a node, read the merged configuration kubelet reports with kubectl pointed at the Rack through [Direct Kubernetes Access](/management/direct-k8s-access). Reading it needs `get` on the `nodes/proxy` subresource:
  ```bash
  $ kubectl get --raw /api/v1/nodes/<node>/proxy/configz | jq '.kubeletconfig | {registryPullQPS, registryBurst}'
  {"registryPullQPS":20,"registryBurst":40}
  ```
  kubelet defaults the pair to `5` and `10`, so an unset Rack reads `{"registryPullQPS":5,"registryBurst":10}` and the command never returns `null`. The on-disk kubelet configuration layout differs across EKS AMI releases, so read this merged view rather than a file on the node.
- Consider your specific environment's needs and your registry's capabilities when adjusting this parameter:
  - For on-premises or self-hosted registries, higher values might be appropriate.
  - For public registries with rate limiting (like Docker Hub), be cautious about setting values too high.
- The relationship between QPS and burst is important: the burst value should always be greater than or equal to the QPS value to allow for effective rate limiting. See [kubelet_registry_pull_qps](/configuration/rack-parameters/aws/kubelet_registry_pull_qps).

## See Also

- [kubelet_registry_pull_qps](/configuration/rack-parameters/aws/kubelet_registry_pull_qps) for the companion steady-state QPS limit that pairs with this burst rate
- [ecr_docker_hub_cache](/configuration/rack-parameters/aws/ecr_docker_hub_cache) for eliminating Docker Hub pulls entirely by caching upstream images through ECR
- [docker_hub_username](/configuration/rack-parameters/aws/docker_hub_username) for authenticating Docker Hub pulls to raise the rate limit

