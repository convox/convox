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

## Error Handling and Retry Behavior

Starting with version 3.23.2, the cleanup process includes improved error handling for ECR image removal:

- **Atomic Operations**: Releases are only removed when their corresponding ECR images have been successfully deleted. If an error occurs during ECR image removal, the associated Convox release will not be deleted.
- **Automatic Retry**: When ECR image removal fails, the cleanup will automatically retry on the next scheduled interval rather than skipping the release permanently.
- **Error Visibility**: Any errors encountered during image removal are reported in the Convox API pod logs, making it easier to diagnose and resolve issues such as ECR permission problems or service disruptions.

To view cleanup errors, check the Convox API pod logs:
```html
$ convox rack logs -r rackName | grep -i "ecr\|cleanup\|release"
```

If releases are not being cleaned up as expected, review the API pod logs to identify any underlying issues preventing ECR image removal.

## Best Practices
- **Production Environments**: Set a higher retention value (50-100) to maintain adequate rollback options.
- **Development Environments**: Use lower values (10-20) to minimize storage costs.
- **High-Frequency Deployments**: Consider lower retention values if you deploy multiple times per day.
- **Compliance Requirements**: Align retention values with your organization's disaster recovery and audit policies.
- **Cost Optimization**: Monitor ECR storage costs and adjust retention accordingly.
- **Monitor Cleanup Logs**: Periodically review API pod logs to ensure cleanup operations are completing successfully.

## Important Considerations
- Ensure that the retention value aligns with your disaster recovery and rollback procedures.
- Once releases are cleaned up, they cannot be recovered.
- The cleanup process is permanent and removes both release metadata and container images.
- Consider your deployment frequency when setting this value - more frequent deployments may require lower retention values.
- If cleanup operations are failing, check the Convox API pod logs for error details before assuming the feature is not working.

## Troubleshooting

### Releases Not Being Cleaned Up
If releases are not being removed as expected:

1. **Check API Pod Logs**: Review the Convox API pod logs for errors related to ECR image removal.
2. **Verify ECR Permissions**: Ensure the rack has appropriate permissions to delete images from ECR repositories.
3. **Confirm Parameter Setting**: Verify the parameter is set correctly with `convox rack params -r rackName`.
4. **Check Cleanup Interval**: The cleanup runs on a schedule defined by `releases_to_retain_task_run_interval_hour` (default: 24 hours).
5. **Active Release Requirement**: Applications without an active release will be skipped during cleanup.

### Common ECR Errors
- **Access Denied**: The rack IAM role may lack permissions to delete ECR images.
- **Image Not Found**: The image may have already been manually deleted from ECR.
- **Rate Limiting**: AWS may be throttling ECR API requests during high-volume cleanup operations.

## Related Parameters
- [releases_to_retain_task_run_interval_hour](/configuration/rack-parameters/aws/releases_to_retain_task_run_interval_hour): Controls how frequently the cleanup task runs to remove old releases.

## Version Requirements
- Basic release cleanup functionality requires at least Convox rack version `3.22.3`.
- Improved error handling and retry behavior requires at least Convox rack version `3.23.2`.
