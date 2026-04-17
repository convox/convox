---
title: "Instance"
slug: instance
url: /reference/primitives/rack/instance
---
# Instance

An Instance provides capacity for running [Processes](/reference/primitives/app/process)

## List Instances
```bash
    $ convox rack instances
    ID                            STATUS   STARTED         PS  CPU    MEM    PUBLIC         PRIVATE
    ip-10-1-1-1.ec2.internal      running  10 minutes ago  8   0.00%  0.00%  18.200.200.99  10.1.1.1
    ip-10-1-2-2.ec2.internal      running  10 minutes ago  6   0.00%  0.00%  3.80.240.200   10.1.2.2
    ip-10-1-3-3.ec2.internal      running  10 minutes ago  8   0.00%  0.00%  3.90.90.200    10.1.3.3
```

## Attributes

| Attribute  | Description                                    |
| ---------- | ---------------------------------------------- |
| **id**     | The unique identifier of the instance          |
| **status** | The current status of the instance (running)   |

## Management

Instance types and counts are managed through [Rack Parameters](/configuration/rack-parameters). Use `node_type` to change the instance size and scaling parameters to control the number of instances in your cluster.

### Connecting to an Instance
```bash
$ convox instances ssh i-0a1b2c3d4e5f
```

### Terminating an Instance
```bash
$ convox instances terminate i-0a1b2c3d4e5f
```
Terminated instances are automatically replaced by the cluster autoscaler.

### Termination Behavior

> On Kubernetes-based (v3) racks, `convox instances terminate` is implemented in rack version 3.24.4 and later. Earlier v3 rack versions return `ERROR: unimplemented`.

On Kubernetes-based (v3) racks, `convox instances terminate` performs a safe, drain-aware sequence:

1. **Cordon** — marks the node `Unschedulable` so new pods are not scheduled onto it
2. **Drain** — removes pods from the node according to the node's readiness state:
   - **Ready nodes** use the Kubernetes Eviction API (`policy/v1`), which respects PodDisruptionBudgets. PDB-blocked evictions retry every 5 seconds for up to a 5-minute deadline, then fall back to force-delete
   - **NotReady nodes** force-delete pods immediately, since kubelet on a NotReady node cannot process evictions
   - DaemonSet-managed pods and mirror pods are skipped during drain, matching `kubectl drain` semantics
3. **Delete** — removes the Kubernetes node object
4. **Terminate cloud VM** — on AWS, the underlying EC2 instance is terminated via `ec2:TerminateInstances`. On other v3 providers, only the Kubernetes node is removed and the cloud VM must be terminated through the provider's own tools

**Per-provider cloud VM reclamation:**

| Provider     | Cordon + Drain | K8s Node Deletion | Cloud VM Termination |
|--------------|----------------|-------------------|----------------------|
| AWS          | Yes            | Yes               | Yes (EC2)            |
| GCP          | Yes            | Yes               | No (manual)          |
| Azure        | Yes            | Yes               | No (manual)          |
| DigitalOcean | Yes            | Yes               | No (manual)          |
| Metal        | Yes            | Yes               | No (manual)          |
| Local        | Yes            | Yes               | N/A                  |

### Cleaning Up a Stuck or NotReady Node

Use `convox instances terminate` to clean up a node left behind after an EKS upgrade that could not drain — for example, a node running an older kubelet version that failed during a rolling upgrade. Stuck `NotReady` nodes can also cause subsequent rack updates to fail.

Identify the stuck node:

```bash
$ convox instances -r rackName
ID                             STATUS     STARTED       PS   CPU     MEM
ip-10-0-1-42.ec2.internal      NotReady   2 days ago    0    0.00%   0.00%
ip-10-0-1-87.ec2.internal      Ready      3 hours ago   8    12.40%  34.10%
```

Terminate it:

```bash
$ convox instances terminate ip-10-0-1-42.ec2.internal -r rackName
OK
```

On an AWS rack this cordons the node, force-deletes any remaining non-DaemonSet/non-mirror pods (since the node is NotReady), deletes the Kubernetes node object, and terminates the underlying EC2 instance. On other v3 providers, the same cordon/drain/node-deletion flow runs and the cloud VM is left in place.
