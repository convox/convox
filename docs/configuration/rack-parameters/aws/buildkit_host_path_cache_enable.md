---
title: "buildkit_host_path_cache_enable"
description: "The buildkit_host_path_cache_enable AWS rack parameter persists the BuildKit layer cache on node local disk so repeat Builds on the same node reuse cached layers, defaulting to false."
slug: buildkit_host_path_cache_enable
url: /configuration/rack-parameters/aws/buildkit_host_path_cache_enable
---

# buildkit_host_path_cache_enable

## Description

The `buildkit_host_path_cache_enable` parameter persists the BuildKit layer cache on the local disk of the node that runs each Build. When enabled, the Rack mounts a hostPath volume from the node (`/mnt/buildkit-cache`, created automatically if it does not exist) into every Build Process at `/var/lib/buildkit`. Each Build imports the BuildKit layer cache from that directory before building and exports the updated cache back to it when the Build completes.

Because the cache lives on the node's disk rather than inside the Build container, it outlives the Build Process. Repeat Builds that schedule onto the same node reuse previously built layers directly from local disk, which can significantly reduce Build times for Apps with large images or many unchanged layers.

The cache is node local. It is not shared between nodes, a Build that lands on a different node starts from that node's cache (or an empty one), and the cache is lost when the node is terminated or replaced. The node cache works alongside the registry-based Build cache that AWS Racks store in the App's ECR repository by default; when both are active, BuildKit imports from both sources and exports to both.

## Default Value

The default value is `false`. At the default, the BuildKit cache directory inside the Build container is ephemeral and is discarded when the Build Process exits.

## Use Cases

- **Faster repeat Builds**: reuse cached layers from local disk instead of rebuilding them or pulling cache blobs from ECR over the network.
- **Large images**: Apps with heavy base layers or long dependency-install steps benefit most, since those layers are read straight from node disk on the next Build.
- **Dedicated build nodes**: pair with [build_node_enabled](/configuration/rack-parameters/aws/build_node_enabled) so Builds schedule onto a small set of dedicated nodes, increasing the chance that consecutive Builds hit a warm cache.

## Setting Parameters

To enable the node-local BuildKit cache, use the following command:
```bash
$ convox rack params set buildkit_host_path_cache_enable=true -r rackName
Updating parameters... OK
```

Applying the change performs one rolling update of the Rack API Deployment. No nodes cycle and no App Processes restart. Builds started after the update completes mount the node cache.

Setting the parameter back to `false` stops mounting the cache into new Build Processes. Cache directories already written to node disk remain until the nodes are replaced.

## Additional Information

- **Availability**: AWS Racks only, available since Rack version `3.23.4`.
- **Validation**: boolean. The CLI rejects non-boolean values with `param 'buildkit_host_path_cache_enable' must be 'true' or 'false' (got "<value>")`.
- **Disk usage**: the Rack does not prune the cache directory. Disk usage at `/mnt/buildkit-cache` grows as Builds run and is reclaimed only when the node is replaced. Size node volumes accordingly on Racks with frequent Builds.
- **Node locality**: cache hit rates depend on Builds landing on the same node. Racks without dedicated build nodes spread Builds across the cluster, so cache reuse is less predictable.
- **Registry cache interaction**: enabling this parameter does not disable the default ECR registry Build cache. If the registry cache is disabled via [disable_image_manifest_cache](/configuration/rack-parameters/aws/disable_image_manifest_cache), the node cache becomes the only Build cache that persists across Builds.

## See Also

- [build_node_enabled](/configuration/rack-parameters/aws/build_node_enabled)
- [build_node_type](/configuration/rack-parameters/aws/build_node_type)
- [disable_image_manifest_cache](/configuration/rack-parameters/aws/disable_image_manifest_cache)
- [ecr_docker_hub_cache](/configuration/rack-parameters/aws/ecr_docker_hub_cache)
