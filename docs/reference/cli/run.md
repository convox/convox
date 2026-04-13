---
title: "run"
slug: run
url: /reference/cli/run
---
# run

## run

Execute a command in a new process

### Usage
```bash
    convox run <service> <command>
```

### Flags

| Flag | Type | Description |
| ---- | ---- | ----------- |
| `--cpu` | number | CPU request in millicores |
| `--cpu-limit` | number | CPU limit in millicores |
| `--detach` | bool | Run in detached mode |
| `--entrypoint` | string | Override the entrypoint |
| `--gpu` | number | Number of GPUs to allocate (requires rack >= 3.21.3) |
| `--memory` | number | Memory request in MB |
| `--memory-limit` | number | Memory limit in MB |
| `--node-labels` | string | Node labels for targeting specific node groups (requires rack >= 3.21.3) |
| `--release` | string | Run against a specific release |
| `--use-service-volume` | bool | Attach all service-configured volumes to the run pod (requires rack >= 3.22.3) |

### Examples

Basic usage:
```bash
    $ convox run web sh
    /usr/src/app #
```

Run against a specific release:
```bash
    $ convox run --release RABCDEFGHIJ web sh
    /usr/src/app #
```

## GPU Support

The `--gpu` flag allows you to request GPU resources for one-off processes. This is particularly useful for machine learning tasks, batch processing, or testing GPU-accelerated code without modifying your service definitions.

### Request a GPU
```bash
    $ convox run web python train-model.py --gpu 1
```

### Target GPU-enabled node groups
When you have configured dedicated GPU node groups in your rack, you can ensure your GPU workloads run on the appropriate hardware:

```bash
    $ convox run web python train-model.py --gpu 1 --node-labels "convox.io/label=gpu-nodes"
```

This works seamlessly with custom node group configurations. For example, if you've set up GPU nodes:

```bash
    $ convox rack params set 'additional_node_groups_config=[{"id":201,"type":"g4dn.xlarge","capacity_type":"ON_DEMAND","label":"gpu-nodes"}]' -r rackName
```

### GPU Use Cases
- **Development Testing**: Quickly test GPU-accelerated code without redeploying
- **Model Training**: Run ML training jobs on demand
- **Batch Processing**: Process computationally intensive workloads occasionally
- **Diagnostics**: Run GPU diagnostics or benchmarking tools

## Automatic Node Placement

When a Service has `nodeSelectorLabels` configured in `convox.yml`, `convox run` automatically inherits those labels as node placement constraints. The run pod targets the same nodes as the deployed Service, including `dedicated-node` tolerations for pools using `convox.io/nodepool` or `convox.io/label`.

For example, if your `convox.yml` has:

```yaml
services:
  gpu-worker:
    build: .
    nodeSelectorLabels:
      convox.io/nodepool: gpu
```

Then `convox run gpu-worker bash` automatically runs on the `gpu` pool — no `--node-labels` flag needed.

### Override with `--node-labels`

To send a run pod to a different node pool (for example, to debug a GPU service on general-purpose nodes):

```bash
    $ convox run gpu-worker bash --node-labels "convox.io/nodepool=workload"
```

This clears the inherited placement and applies the specified labels instead.

### Clear inherited node placement

To remove the inherited node affinity and allow the pod to schedule on general cluster nodes:

```bash
    $ convox run gpu-worker bash --node-labels ""
```

This is useful for debugging when you want to run a one-off process outside its usual dedicated pool.

> Builds are not affected by automatic node placement — `convox build` always uses the configured build nodes regardless of `nodeSelectorLabels`.

## Service Volume Support

The `--use-service-volume` flag enables one-off processes to access the same persistent volumes configured for the service. This ensures data consistency and enables maintenance operations that require access to persistent storage.

### Access service volumes
```bash
    $ convox run web sh -a myapp --use-service-volume
```

This flag automatically maps all volumes configured in your service definition to the run pod, including:
- EFS volumes for shared storage
- emptyDir volumes for temporary storage
- Any other volume types configured in your `convox.yml`

### Volume Use Cases
- **Database Migrations**: Run migration scripts that need access to shared configuration files
- **Batch Jobs**: Execute jobs that process data stored on persistent volumes
- **Debugging**: Inspect and troubleshoot volume-mounted data through interactive shells
- **Maintenance**: Perform cleanup or data manipulation tasks on persistent storage
- **Zero-Scale Services**: Access volumes for services that are scaled to zero

### Example with EFS Volume
If your service is configured with an EFS volume:

```yaml
services:
  web:
    volumeOptions:
      - awsEfs:
          id: "efs-1"
          accessMode: ReadWriteMany
          mountPath: "/data"
```

Running with `--use-service-volume` ensures the `/data` directory is available in your one-off process:

```bash
    $ convox run web ls /data --use-service-volume
    file1.txt
    file2.txt
    shared-config.json
```

## Advanced Examples

### Combine resource requests with volumes
```bash
    $ convox run web python process.py --cpu 2000 --memory 4096 --use-service-volume
```

### GPU workload with specific node targeting
```bash
    $ convox run worker python train.py --gpu 2 --node-labels "convox.io/label=ml-nodes" --memory 8192
```

### Detached process with volumes
```bash
    $ convox run background-job ./long-running-task.sh --detach --use-service-volume
```

## Version Requirements

- Basic `convox run` functionality: All versions
- GPU support (`--gpu`, `--node-labels`): Requires CLI and rack version >= 3.21.3
- Volume support (`--use-service-volume`): Requires CLI and rack version >= 3.22.3
- Automatic node placement (inherits `nodeSelectorLabels`): Requires CLI and rack version >= 3.24.3

## See Also

- [One-off Commands](/management/run) for run command patterns
- [Workload Placement](/configuration/scaling/workload-placement) for `nodeSelectorLabels` configuration
- [Karpenter](/configuration/scaling/karpenter) for dedicated pool isolation with `dedicated: true`