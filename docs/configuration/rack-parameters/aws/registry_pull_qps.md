---
title: "registry_pull_qps"
draft: false
slug: registry_pull_qps
url: /configuration/rack-parameters/aws/registry_pull_qps
---

# registry_pull_qps

## Description
The `registry_pull_qps` parameter controls the maximum number of image pull requests that can be made to a container registry per second. This parameter helps to regulate image pull traffic and prevent overwhelming the registry.

## Default Value
The default value for `registry_pull_qps` is `5`.

## Use Cases
- **Prevent registry overload**: Limit the number of concurrent image pulls to avoid overwhelming the registry.
- **Optimize resource utilization**: Manage image pull traffic to optimize resource usage on the node.

## Setting Parameters
To enable the `registry_pull_qps` parameter, use the following command:
```html
$ convox rack params set registry_pull_qps=value -r rackName
Setting parameters... OK
```

Replace value with the desired number of image pull requests per second.

## Additional Information
A higher `registry_pull_qps` value can improve image pull performance but may increase the load on the registry. A lower value can help prevent registry overload but may impact pod startup time. It's essential to find the optimal value based on your cluster's workload and registry capacity.