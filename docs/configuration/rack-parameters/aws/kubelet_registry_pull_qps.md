---
title: "kubelet_registry_pull_qps"
description: "The kubelet_registry_pull_qps AWS rack parameter caps the image pull requests to a container registry per second, defaulting to 5."
slug: kubelet_registry_pull_qps
url: /configuration/rack-parameters/aws/kubelet_registry_pull_qps
---

# kubelet_registry_pull_qps

## Description
The `kubelet_registry_pull_qps` parameter controls the maximum number of image pull requests that can be made to a container registry per second. This parameter helps to regulate image pull traffic and prevent overwhelming the registry.

## Default Value
The default value for `kubelet_registry_pull_qps` is `5`.

This parameter works together with [kubelet_registry_burst](/configuration/rack-parameters/aws/kubelet_registry_burst), which controls the maximum burst rate for image pulls, allowing temporary exceedance of the QPS limit.

## Use Cases
- **Prevent registry overload**: Limit the number of concurrent image pulls to avoid overwhelming the registry.
- **Optimize resource utilization**: Manage image pull traffic to optimize resource usage on the node.
- **High Deployment Frequency**: Increase limits in environments with frequent container deployments.
- **Large Container Images**: Optimize pull rates for environments with large image sizes.
- **Registry Rate Limiting**: Adjust limits to prevent hitting registry-imposed rate limits.
- **Cluster Scale-Up**: Improve node startup time by allowing faster concurrent image pulls.
- **CI/CD Optimization**: Accelerate deployments in continuous integration/deployment pipelines.

## Setting Parameters
To enable the `kubelet_registry_pull_qps` parameter, use the following command:
```bash
$ convox rack params set kubelet_registry_pull_qps=value -r rackName
Setting parameters... OK
```

Replace value with the desired number of image pull requests per second.

## Additional Information
- A higher `kubelet_registry_pull_qps` value can improve image pull performance but may increase the load on the registry. A lower value can help prevent registry overload but may impact pod startup time. It's essential to find the optimal value based on your cluster's workload and registry capacity.
- This parameter affects all nodes in your cluster and applies to all image pull operations.
- Changes to this parameter require a node rotation to take effect on existing nodes, which happens automatically when the parameter is updated.
- You can verify the current setting on a node by SSH-ing into it and examining the kubelet configuration:
  ```bash
  $ sudo cat /etc/kubernetes/kubelet/kubelet-config.json | grep -E "registryPullQPS|registryBurst"
  ```
- Consider your specific environment's needs and your registry's capabilities when adjusting this parameter:
  - For on-premises or self-hosted registries, higher values might be appropriate.
  - For public registries with rate limiting (like Docker Hub), be cautious about setting values too high.
- The relationship between QPS and burst is important: the burst value should always be greater than or equal to the QPS value to allow for effective rate limiting. See [kubelet_registry_burst](/configuration/rack-parameters/aws/kubelet_registry_burst).

## See Also

- [kubelet_registry_burst](/configuration/rack-parameters/aws/kubelet_registry_burst) for the companion burst-rate limit that pairs with this QPS limit
- [ecr_docker_hub_cache](/configuration/rack-parameters/aws/ecr_docker_hub_cache) for eliminating Docker Hub pulls entirely by caching upstream images through ECR
- [docker_hub_username](/configuration/rack-parameters/aws/docker_hub_username) for authenticating Docker Hub pulls to raise the rate limit