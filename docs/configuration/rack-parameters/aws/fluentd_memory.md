---
title: "fluentd_memory"
description: "The fluentd_memory AWS rack parameter configures the memory request and limit for the Fluentd log collector DaemonSet, defaulting to 300Mi."
slug: fluentd_memory
url: /configuration/rack-parameters/aws/fluentd_memory
---

# fluentd_memory

## Description
The `fluentd_memory` parameter configures the memory request and limit for the Fluentd log collector DaemonSet. Fluentd runs on every node in the cluster and forwards application and system logs to CloudWatch. Both the Kubernetes memory request and limit are set to the same value, giving the Fluentd pod a Guaranteed QoS class.

## Default Value
The default value for `fluentd_memory` is `300Mi`.

## Use Cases
- **High Log Throughput**: Increase memory for racks with many services, verbose application logging, or high request rates to prevent Fluentd OOM restarts and log loss.
- **GPU / ML Workloads**: Workloads generating large volumes of stdout/stderr may require `512Mi` or `1Gi`.
- **Resource Optimization**: Reduce memory below `300Mi` on racks with low log volume to reclaim node resources.

## Setting Parameters
To set the `fluentd_memory` parameter, use the following command:
```bash
$ convox rack params set fluentd_memory=512Mi -r rackName
Updating parameters... OK
```
This command sets the Fluentd memory allocation to 512Mi.

## Additional Information
The value must be a valid Kubernetes memory string matching the pattern `^\d+(Mi|Gi)$` (e.g., `200Mi`, `512Mi`, `1Gi`). When Fluentd is OOM-killed due to high log throughput, in-flight log buffers are lost. Increasing this value prevents gaps in log delivery during high-throughput periods.

The default was `200Mi` before the switch to the fluentd 1.19 image (`convox/fluentd:1.19-all`). With jemalloc that image uses about 140–170Mi at steady state and can reach about 200Mi during large log bursts, so the default is now `300Mi`.

## See Also
- [fluentd_disable](/configuration/rack-parameters/aws/fluentd_disable) to disable Fluentd entirely
- [syslog](/configuration/rack-parameters/aws/syslog) for forwarding logs to an external syslog endpoint
- [Logging](/configuration/logging) for an overview of Convox logging
