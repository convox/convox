---
title: "fluentd_memory"
description: "The fluentd_memory Digital Ocean rack parameter sets the memory request and limit for the Fluentd log collector DaemonSet, defaulting to 200Mi."
slug: fluentd_memory
url: /configuration/rack-parameters/do/fluentd_memory
---

# fluentd_memory

## Description
The `fluentd_memory` parameter configures the memory request and limit for the Fluentd log collector DaemonSet. Fluentd runs on every node in the cluster and forwards application and system logs. Both the Kubernetes memory request and limit are set to this value. Fluentd runs at the `system-node-critical` priority, so on a node with no room for it the scheduler evicts lower-priority Pods to place it.

## Default Value
The default value for `fluentd_memory` is `200Mi`.

## Accepted Values
A whole number of `Mi` or `Gi` with no leading zero, such as `200Mi`, `512Mi` or `1Gi`. The CLI rejects any other value:

```text
ERROR: param 'fluentd_memory' must be a whole number of Mi or Gi, such as 200Mi or 1Gi
```

`--force` does not bypass the check, which requires CLI 3.25.12 or later. The Console form accepts the same values. A value in another form set with an older CLI or through `convox rack install`, such as `2.5Gi`, still sets the memory limit, but Fluentd sizes its buffers as for `200Mi` until the value is set again as a whole number of `Mi` or `Gi`.

## Use Cases
- **Output outages and bursts**: Fluentd sizes the buffer for lines it has not yet delivered from this value. A larger value lets it absorb a longer Elasticsearch outage or a bigger burst before it pauses reading container logs.
- **GPU / ML Workloads**: Workloads generating large volumes of stdout/stderr may require `512Mi` or `1Gi`.

## Setting Parameters
To set the `fluentd_memory` parameter, use the following command:
```bash
$ convox rack params set fluentd_memory=512Mi -r rackName
Updating parameters... OK
```
This command sets the Fluentd memory allocation to 512Mi.

## Additional Information
When its buffer is full, Fluentd stops reading container logs until Elasticsearch catches up, and syslog forwarding pauses with it. The lines arrive late rather than being lost, unless the node rotates a container's log file before Fluentd reads it. Values below `200Mi` leave Fluentd little room under load. Changing the value rolls the Fluentd pods on every node.

The Fluentd image in Rack version `3.25.11` uses about 26 MiB more memory at idle than the one before it. Before updating a Rack set below `200Mi` to `3.25.11`, raise `fluentd_memory` to at least `200Mi`, or Fluentd can be OOM-killed repeatedly.

## See Also
- [syslog](/configuration/rack-parameters/do/syslog) for forwarding logs to an external syslog endpoint
- [Logging](/configuration/logging) for an overview of Convox logging
