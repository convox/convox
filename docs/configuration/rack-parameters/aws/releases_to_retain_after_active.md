---
title: "releases_to_retain_after_active"
draft: false
slug: releases_to_retain_after_active
url: /configuration/rack-parameters/aws/releases_to_retain_after_active
---

# releases_to_retain_after_active

## Description
The `releases_to_retain_after_active` parameter specifies the number of releases to retain after the currently active release. This parameter enables automatic cleanup of old application releases and their associated ECR (Elastic Container Registry) images, helping manage storage costs and maintain a cleaner deployment history.

The cleanup process counts releases based on the **active release** (not the latest release), ensuring that your production deployments and their recent history are always preserved. If an application has no active release, the cleanup operation will be skipped for that application.

## Default Value
The default value for `releases_to_retain_after_active` is **unset** (no automatic cleanup). The feature is disabled by default and requires explicit configuration to activate.

## Use Cases
- **Storage Cost Reduction**: Automatically remove old ECR images that can accumulate significant storage costs over time.
- **Resource Management**: Prevent unlimited growth of release history and maintain a manageable deployment history.
- **Repository Cleanup**: Reduce clutter in your ECR repositories by removing unused container images.
- **Automated Maintenance**: Eliminate manual cleanup tasks through automated scheduling.
- **Compliance**: Meet data retention policies that require limiting the number of stored application versions.

## Setting Parameters
To set the `releases_to_retain_after_active` parameter, use the following command:
```html
$ convox rack params set releases_to_retain_after_active=100 -r rackName
Setting parameters... OK
```

This command configures the rack to retain the active release plus 100 releases after it.

### Example Configurations

To retain only the active release plus 50 recent releases:
```html
$ convox rack params set releases_to_retain_after_active=50 -r rackName
```

To enable minimal retention (active release plus 10 recent releases):
```html
$ convox rack params set releases_to_retain_after_active=10 -r rackName
```

## Additional Information
- **Feature Activation**: The cleanup feature is disabled by default and will not remove any releases unless this parameter is explicitly set.
- **Active Release Focus**: The retention count is based on the active release, not the latest release, ensuring production deployments are protected.
- **Application Safety**: Applications without an active release will be skipped during the cleanup process.
- **Comprehensive Cleanup**: Both application releases and their corresponding ECR images will be removed when the threshold is exceeded.
- **Cleanup Scheduling**: The cleanup task runs according to the interval defined by the `releases_to_retain_task_run_interval_hour` parameter (default: 24 hours).
- When a new release becomes active, the retention policy is re-evaluated during the next cleanup cycle.
- The active release itself is always retained regardless of this setting.

## Best Practices
- **Production Environments**: Set a higher retention value (50-100) to maintain adequate rollback options.
- **Development Environments**: Use lower values (10-20) to minimize storage costs.
- **High-Frequency Deployments**: Consider lower retention values if you deploy multiple times per day.
- **Compliance Requirements**: Align retention values with your organization's disaster recovery and audit policies.
- **Cost Optimization**: Monitor ECR storage costs and adjust retention accordingly.

## Important Considerations
- Ensure that the retention value aligns with your disaster recovery and rollback procedures.
- Once releases are cleaned up, they cannot be recovered.
- The cleanup process is permanent and removes both release metadata and container images.
- Consider your deployment frequency when setting this value - more frequent deployments may require lower retention values.

## Related Parameters
- [releases_to_retain_task_run_interval_hour](/configuration/rack-parameters/aws/releases_to_retain_task_run_interval_hour): Controls how frequently the cleanup task runs to remove old releases.

## Version Requirements
This feature requires at least Convox rack version `3.22.3`.