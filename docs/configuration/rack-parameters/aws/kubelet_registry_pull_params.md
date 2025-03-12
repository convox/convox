---
title: "kubelet_registry_pull_qps and kubelet_registry_burst"
draft: false
slug: kubelet_registry_pull_params
url: /configuration/rack-parameters/aws/kubelet_registry_pull_params
---

# kubelet_registry_pull_qps and kubelet_registry_burst

## Description
The `kubelet_registry_pull_qps` and `kubelet_registry_burst` parameters allow you to configure image pull limits within the kubelet configuration on each node in your Kubernetes cluster. These parameters provide control over the rate at which images are pulled from container registries.

- `kubelet_registry_pull_qps`: Controls the steady-state rate limit for image pulls (queries per second).
- `kubelet_registry_burst`: Controls the maximum burst rate for image pulls, allowing temporary exceedance of the QPS limit.

These parameters are particularly useful in environments with high deployment frequencies, large container images, or when working with registries that impose rate limits.

## Default Values
- The default value for `kubelet_registry_pull_qps` is `5`.
- The default value for `kubelet_registry_burst` is `10`.

## Use Cases
- **High Deployment Frequency**: Increase limits in environments with frequent container deployments.
- **Large Container Images**: Optimize pull rates for environments with large image sizes.
- **Registry Rate Limiting**: Adjust limits to prevent hitting registry-imposed rate limits.
- **Cluster Scale-Up**: Improve node startup time by allowing faster concurrent image pulls.
- **CI/CD Optimization**: Accelerate deployments in continuous integration/deployment pipelines.

## Setting Parameters
To configure these parameters, use the following command:
```html
$ convox rack params set kubelet_registry_pull_qps=10 kubelet_registry_burst=20 -r rackName
Setting parameters... OK
```

## Additional Information
- Increasing these values can improve the performance of your cluster during deployments by allowing more images to be pulled simultaneously.
- Setting these values too high could lead to network congestion or excessive load on your container registry.
- These parameters affect all nodes in your cluster and apply to all image pull operations.
- Changes to these parameters require a node rotation to take effect on existing nodes, which happens automatically when the parameters are updated.
- You can verify the current settings on a node by SSH-ing into it and examining the kubelet configuration:
  ```bash
  $ sudo cat /etc/kubernetes/kubelet/kubelet-config.json | grep -E "registryPullQPS|registryBurst"
  ```
- Consider your specific environment's needs and your registry's capabilities when adjusting these parameters:
  - For on-premises or self-hosted registries, higher values might be appropriate.
  - For public registries with rate limiting (like Docker Hub), be cautious about setting values too high.
- The relationship between QPS and burst is important: the burst value should always be greater than or equal to the QPS value to allow for effective rate limiting.

## Version Requirements
This feature requires at least Convox rack version `3.18.9`.
