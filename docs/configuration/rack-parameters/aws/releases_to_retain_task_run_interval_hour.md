---
title: "releases_to_retain_task_run_interval_hour"
draft: false
slug: releases_to_retain_task_run_interval_hour
url: /configuration/rack-parameters/aws/releases_to_retain_task_run_interval_hour
---

# releases_to_retain_task_run_interval_hour

## Description
The `releases_to_retain_task_run_interval_hour` parameter defines the interval in hours at which the automatic release cleanup task runs. This task is responsible for removing old application releases and their associated ECR images that exceed the retention threshold set by the `releases_to_retain_after_active` parameter.

By configuring this interval, you can control how frequently the system checks for and removes outdated releases, balancing between timely cleanup and system resource usage.

## Default Value
The default value for `releases_to_retain_task_run_interval_hour` is `24` hours (once daily).

## Use Cases
- **Cost Management**: Schedule regular cleanup to prevent ECR storage costs from accumulating.
- **Resource Scheduling**: Run cleanup tasks during off-peak hours to minimize impact on production workloads.
- **Storage Optimization**: Control how quickly storage is reclaimed after deployments by adjusting cleanup frequency.
- **Deployment Patterns**: Align cleanup schedules with your deployment frequency and patterns.
- **Maintenance Windows**: Schedule cleanup to coincide with existing maintenance windows.

## Setting Parameters
To set the `releases_to_retain_task_run_interval_hour` parameter, use the following command:
```html
$ convox rack params set releases_to_retain_task_run_interval_hour=12 -r rackName
Setting parameters... OK
```

This command configures the cleanup task to run every 12 hours.

### Example Configurations

For twice-daily cleanup:
```html
$ convox rack params set releases_to_retain_task_run_interval_hour=12 -r rackName
```

For weekly cleanup (every 7 days):
```html
$ convox rack params set releases_to_retain_task_run_interval_hour=168 -r rackName
```

For more frequent cleanup in high-deployment environments:
```html
$ convox rack params set releases_to_retain_task_run_interval_hour=6 -r rackName
```

## Additional Information
- **Prerequisite**: This parameter only takes effect when `releases_to_retain_after_active` is set. Without a retention policy, the cleanup task will not run.
- The interval is specified in hours and must be a positive integer.
- The cleanup task runs as a background process and does not impact running applications.
- The first cleanup task will run after the specified interval from when the parameter is set or the rack is updated.
- The cleanup task only removes releases that exceed the threshold defined by `releases_to_retain_after_active`.
- Applications without an active release are automatically skipped during cleanup.
- Cleanup operations include:
  - Identifying releases beyond the retention threshold
  - Removing release metadata from the cluster
  - Deleting associated container images from ECR
  - Cleaning up related Kubernetes resources

## Best Practices
- **High-frequency deployments** (multiple times per day): Consider setting the interval to 6-12 hours to prevent rapid accumulation.
- **Regular deployments** (daily/weekly): The default 24-hour interval is typically sufficient.
- **Infrequent deployments** (monthly or less): A longer interval (48-168 hours) may be appropriate.
- **Development environments**: Shorter intervals (6-12 hours) can help keep resources clean and costs low.
- **Production environments**: Balance cleanup frequency with operational stability, typically 12-24 hours.
- **Cost-sensitive environments**: More frequent cleanup (6-12 hours) helps minimize ECR storage costs.

## Performance Considerations
- Setting a shorter interval results in more frequent cleanups but may increase system overhead.
- Setting a longer interval reduces system overhead but may delay storage reclamation and cost savings.
- The task execution time varies depending on the number of releases and ECR images to be cleaned up.
- The cleanup process is designed to be non-disruptive and runs with low priority.

## Monitoring
- Monitor your ECR storage costs to determine if the cleanup frequency is appropriate.
- Check rack logs for cleanup task execution and any potential errors.
- Review ECR repository sizes to ensure cleanup is working effectively.
- Adjust the interval based on your observed storage patterns and deployment frequency.

## Related Parameters
- [releases_to_retain_after_active](/configuration/rack-parameters/aws/releases_to_retain_after_active): Defines how many releases to retain after the active release. Must be set for the cleanup task to run.

## Version Requirements
This feature requires at least Convox rack version `3.22.3`.