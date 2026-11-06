---
title: "ecr_immutable_tags_enabled"
description: "The ecr_immutable_tags_enabled AWS rack parameter creates App ECR repositories with immutable image tags, defaulting to false."
slug: ecr_immutable_tags_enabled
url: /configuration/rack-parameters/aws/ecr_immutable_tags_enabled
---

# ecr_immutable_tags_enabled

## Description

The `ecr_immutable_tags_enabled` parameter controls the tag mutability of the ECR repository the Rack creates for each App. When set to `true`, repositories for newly created Apps use `imageTagMutability=IMMUTABLE`, so an image tag, once pushed, cannot be overwritten by a later push. Immutable tags keep a pushed tag resolving to the same image content for as long as the image exists.

Mutability is set once, when the repository is created. Enabling the parameter does not change repositories for existing Apps, and disabling it does not change repositories already created as immutable. Only Apps created while the parameter is `true` receive immutable repositories.

Standard Build, deploy, promote, and rollback flows are unaffected: each Build pushes a unique `<service>.<build id>` tag, which immutable repositories accept normally. From Rack version `3.25.10`, each Build also pushes its registry Build cache under a unique `<service>.buildcache.<build id>` tag.

## Default Value

The default value is `false`. At the default, App repositories are created with mutable tags, identical to previous Rack versions.

## Use Cases

- **Supply-chain integrity**: prevent a pushed image tag from being replaced by a later push, so a Build that was tested and promoted keeps referring to the same image content.
- **Security compliance**: satisfy AWS Foundational Security Best Practices control ECR.2, which checks that private ECR repositories have tag immutability configured.

## Setting Parameters

```bash
$ convox rack params set ecr_immutable_tags_enabled=true -r rackName
Updating parameters... OK
```

Applying the change updates the Rack API configuration and performs one rolling update of the Rack API Deployment. No nodes cycle and no App Processes restart. Once the Rack update completes, repositories for newly created Apps are immutable.

Setting the parameter back to `false` restores mutable repository creation for new Apps. Repositories created while the parameter was `true` remain immutable.

## Additional Information

This parameter is available on AWS Racks only and requires Rack version `3.24.10` or later.

- **Validation:** boolean. The CLI rejects non-boolean values with `param 'ecr_immutable_tags_enabled' must be 'true' or 'false' (got "<value>")`. Accepted variants such as `1` or `TRUE` are stored as `true` or `false`.
- **Build cache interaction:** with the parameter `true`, the image-manifest registry Build cache, enabled by default on AWS Racks, exports each Service's cache to a `<service>.buildcache.<build id>` tag, and each Build imports the cache of the App's most recent completed Build. When that Build left no cache tag, as for an App's first Build after the Rack updates to `3.25.10`, the Build imports the existing `<service>.buildcache` tag if there is one. No tag is overwritten, so the repository stays `IMMUTABLE`. A Service built from `image:`, or from a Dockerfile with no build steps of its own, exports no cache. With the parameter `false`, Builds use the single `<service>.buildcache` tag, which on a repository created immutable earlier is written once and never refreshed. On Racks before `3.25.10` an immutable repository writes the `<service>.buildcache` tag once and never refreshes it, so Builds reuse the first Build's cache; on those Racks, pair this parameter with `disable_image_manifest_cache=true` if Build cache matters.
- **Storage:** with the parameter `true`, each Build adds one cache image per Service next to its Build image. [`releases_to_retain_after_active`](/configuration/rack-parameters/aws/releases_to_retain_after_active) removes a Build's cache images together with the Build. An ECR lifecycle rule that keeps the last N images counts cache images too, so it keeps about half as many Builds as before.
- **Existing repositories:** the Rack does not manage tag mutability after repository creation. To change an existing App repository, use the ECR console or `aws ecr put-image-tag-mutability`.
- **Downgrade safety:** downgrading the Rack below `3.24.10` removes the parameter and restores mutable repository creation for new Apps. Repositories already created as immutable keep that setting. Downgrading below `3.25.10` sends Builds back to the single `<service>.buildcache` tag, which on an immutable repository holds the cache from before `3.25.10`. Cache images already pushed under `<service>.buildcache.<build id>` stay until the App is deleted or you remove them.

## See Also

- [ecr_scan_on_push_enable](/configuration/rack-parameters/aws/ecr_scan_on_push_enable)
- [disable_image_manifest_cache](/configuration/rack-parameters/aws/disable_image_manifest_cache)
- [releases_to_retain_after_active](/configuration/rack-parameters/aws/releases_to_retain_after_active)
- [ecr_full_access](/configuration/rack-parameters/aws/ecr_full_access)
- [ecr_additional_policy_arn](/configuration/rack-parameters/aws/ecr_additional_policy_arn)
