---
title: "disable_image_manifest_cache"
description: "The disable_image_manifest_cache AWS rack parameter turns off the registry-backed BuildKit layer cache stored in each App's ECR repository, defaulting to false."
slug: disable_image_manifest_cache
url: /configuration/rack-parameters/aws/disable_image_manifest_cache
---

# disable_image_manifest_cache

## Description

The `disable_image_manifest_cache` parameter controls the registry-backed Build cache that AWS Racks use by default. During each Build, BuildKit exports its layer cache as an OCI image manifest to the App's ECR repository under a per-Service `<service>.buildcache` tag, and imports that cache at the start of the next Build. This lets Builds reuse layers across build Processes and build nodes, since the cache lives in ECR rather than on any single node.

When set to `true`, the Rack stops exporting and importing the ECR registry cache. Builds fall back to a layer cache local to the build Process, which does not persist after the Build completes unless the [`buildkit_host_path_cache_enable`](/configuration/rack-parameters/aws/buildkit_host_path_cache_enable) parameter separately mounts a persistent cache directory on the build node.

## Default Value

The default value is `false`. At the default, the ECR registry Build cache is active on AWS Racks.

## Use Cases

- **Immutable ECR tags**: with `ecr_immutable_tags_enabled=true`, the `buildcache` tag is written once and can never be refreshed, so the registry cache stops updating. Disable it on Racks that use immutable tags.
- **ECR storage costs**: the cache export uses `mode=max`, which pushes intermediate layers for every Build stage. Disabling the cache stops these pushes and keeps App repositories limited to deployable images.
- **Repository content policies**: keep App ECR repositories free of cache artifacts when lifecycle rules or compliance scanning do not account for the `buildcache` tag.
- **Stale cache troubleshooting**: rule out cache reuse when debugging unexpected Build output. For a one-off Build without cache, `convox build --no-cache` avoids a Rack parameter change.

## Setting Parameters

```bash
$ convox rack params set disable_image_manifest_cache=true -r rackName
Updating parameters... OK
```

Applying the change updates the Rack API configuration and performs one rolling update of the Rack API Deployment. No nodes cycle and no App Processes restart. Builds started after the Rack update completes use the new setting.

Setting the parameter back to `false` restores the ECR registry cache on subsequent Builds:

```bash
$ convox rack params set disable_image_manifest_cache=false -r rackName
Updating parameters... OK
```

## Additional Information

This parameter applies to AWS Racks only and is available since Rack version `3.14.0`.

- **Validation:** boolean. The CLI rejects non-boolean values with `param 'disable_image_manifest_cache' must be 'true' or 'false' (got "<value>")`. Accepted variants such as `1` or `TRUE` are stored as `true` or `false`.
- **Cache location:** the cache is stored in the same ECR repository as the App's images, tagged `<service>.buildcache`. It is exported with `mode=max`, OCI media types, and estargz compression, and cache push or pull failures never fail the Build.
- **Existing cache images:** disabling the parameter stops refreshing the `buildcache` tag but does not delete cache images already in ECR. Remove them with the ECR console or `aws ecr batch-delete-image` if you want to reclaim storage.
- **Scope:** this parameter only affects the Build layer cache. It has no effect on image pulls, deploys, or the ECR pull-through cache managed by `ecr_docker_hub_cache`.

## See Also

- [ecr_immutable_tags_enabled](/configuration/rack-parameters/aws/ecr_immutable_tags_enabled) for immutable App repository tags, which freeze the `buildcache` tag and make disabling this cache the recommended pairing
- [buildkit_host_path_cache_enable](/configuration/rack-parameters/aws/buildkit_host_path_cache_enable) for a node-local persistent Build cache that works with the registry cache disabled
- [ecr_docker_hub_cache](/configuration/rack-parameters/aws/ecr_docker_hub_cache) for the unrelated ECR pull-through cache used by resource images
