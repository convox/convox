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

Standard Build, deploy, promote, and rollback flows are unaffected: each Build pushes a unique `<service>.<build id>` tag, which immutable repositories accept normally.

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
- **Build cache interaction:** the image-manifest registry Build cache, enabled by default on AWS Racks, stores its cache under a fixed per-Service `buildcache` tag in the App repository. On an immutable repository that tag is written once and never refreshed, so Builds still succeed but reuse the first Build's cache snapshot, and cache effectiveness degrades as the code changes. On cache-sensitive Racks, pair this parameter with `disable_image_manifest_cache=true` or leave immutable tags disabled.
- **Existing repositories:** the Rack does not manage tag mutability after repository creation. To change an existing App repository, use the ECR console or `aws ecr put-image-tag-mutability`.
- **Downgrade safety:** downgrading the Rack below `3.24.10` removes the parameter and restores mutable repository creation for new Apps. Repositories already created as immutable keep that setting.

## See Also

- [ecr_scan_on_push_enable](/configuration/rack-parameters/aws/ecr_scan_on_push_enable)
- [ecr_full_access](/configuration/rack-parameters/aws/ecr_full_access)
- [ecr_additional_policy_arn](/configuration/rack-parameters/aws/ecr_additional_policy_arn)
