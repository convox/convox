---
title: "kubelet_registry_burst"
description: "The kubelet_registry_burst AWS rack parameter sets the maximum image pull requests allowed in a burst beyond registry_pull_qps, defaulting to 10."
slug: kubelet_registry_burst
url: /configuration/rack-parameters/aws/kubelet_registry_burst
---

# kubelet_registry_burst

## Description
The `kubelet_registry_burst` parameter defines the maximum number of image pull requests that can be made in a burst, exceeding the `registry_pull_qps` limit for a short duration. This parameter allows for short-lived spikes in image pull traffic.

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
To enable the `kubelet_registry_burst` parameter, use the following command:
```bash
$ convox rack params set kubelet_registry_burst=value -r rackName
Setting parameters... OK
```

Replace value with the desired maximum number of burst image pull requests.

## Additional Information
- The `kubelet_registry_burst` parameter complements `kubelet_registry_pull_qps` by providing flexibility in handling short-lived spikes in image pull traffic. However, excessive burst values can still overload the registry. It's essential to consider the average image pull rate and the expected peak load when setting this value.
- This parameter affects all nodes in your cluster and applies to all image pull operations.
- Changes to this parameter require a node rotation to take effect on existing nodes, which happens automatically when the parameter is updated.
- You can verify the current setting on a node by SSH-ing into it and examining the kubelet configuration:
  ```bash
  $ sudo cat /etc/kubernetes/kubelet/kubelet-config.json | grep -E "registryPullQPS|registryBurst"
  ```
- Consider your specific environment's needs and your registry's capabilities when adjusting this parameter:
  - For on-premises or self-hosted registries, higher values might be appropriate.
  - For public registries with rate limiting (like Docker Hub), be cautious about setting values too high.
- The relationship between QPS and burst is important: the burst value should always be greater than or equal to the QPS value to allow for effective rate limiting. See [kubelet_registry_pull_qps](/configuration/rack-parameters/aws/kubelet_registry_pull_qps).

## See Also

- [kubelet_registry_pull_qps](/configuration/rack-parameters/aws/kubelet_registry_pull_qps) for the companion steady-state QPS limit that pairs with this burst rate
- [ecr_docker_hub_cache](/configuration/rack-parameters/aws/ecr_docker_hub_cache) for eliminating Docker Hub pulls entirely by caching upstream images through ECR
- [docker_hub_username](/configuration/rack-parameters/aws/docker_hub_username) for authenticating Docker Hub pulls to raise the rate limit

