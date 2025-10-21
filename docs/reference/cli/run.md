---
title: "run"
draft: false
slug: run
url: /reference/cli/run
---
# run

## run

Execute a command in a new process

### Usage
```html
    convox run <service> <command>
```

### Flags

 - `--app`: String. Specifies the app name
 - `--cpu`: Number. Specifies the millicpu units of requests to set for the process
 - `--cpu-limit`: Number. Specifies the millicpu units of limit to set for the process
 - `--detach`: Boolean. To run in detach mode
 - `--entrypoint`: String. Specifies the entrypoint
 - `--memory`: Number. Specifies the memory megabytes of requests to set for the process
 - `--memory-limit`: Number. Specifies the memory megabytes of limit to set for the process
 - `--rack`: String. Specifies the rack name
 - `--release`: String. Specifies the release
 - `--gpu`: Number. Specifies the number of GPUs to allocate for the process (requires version >= 3.21.3)
 - `--node-labels`: String. Specifies node labels for targeting specific node groups (requires version >= 3.21.3)
 - `--use-service-volume`: Boolean. Attaches all service-configured volumes to the run pod (requires version >= 3.22.3)

### Examples

Basic usage:
```html
    $ convox run web sh
    /usr/src/app #
```

Run against a specific release:
```html
    $ convox run --release RABCDEFGHIJ web sh
    /usr/src/app #
```

## GPU Support

The `--gpu` flag allows you to request GPU resources for one-off processes. This is particularly useful for machine learning tasks, batch processing, or testing GPU-accelerated code without modifying your service definitions.

### Request a GPU
```html
    $ convox run web python train-model.py --gpu 1
```

### Target GPU-enabled node groups
When you have configured dedicated GPU node groups in your rack, you can ensure your GPU workloads run on the appropriate hardware:

```html
    $ convox run web python train-model.py --gpu 1 --node-labels "convox.io/label=gpu-nodes"
```

This works seamlessly with custom node group configurations. For example, if you've set up GPU nodes:

```html
    $ convox rack params set 'additional_node_groups_config=[{"id":201,"type":"g4dn.xlarge","capacity_type":"ON_DEMAND","label":"gpu-nodes"}]' -r rackName
```

### GPU Use Cases
- **Development Testing**: Quickly test GPU-accelerated code without redeploying
- **Model Training**: Run ML training jobs on demand
- **Batch Processing**: Process computationally intensive workloads occasionally
- **Diagnostics**: Run GPU diagnostics or benchmarking tools

## Service Volume Support

The `--use-service-volume` flag enables one-off processes to access the same persistent volumes configured for the service. This ensures data consistency and enables maintenance operations that require access to persistent storage.

### Access service volumes
```html
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

```html
    $ convox run web ls /data --use-service-volume
    file1.txt
    file2.txt
    shared-config.json
```

## Advanced Examples

### Combine resource requests with volumes
```html
    $ convox run web python process.py --cpu 2000 --memory 4096 --use-service-volume
```

### GPU workload with specific node targeting
```html
    $ convox run worker python train.py --gpu 2 --node-labels "convox.io/label=ml-nodes" --memory 8192
```

### Detached process with volumes
```html
    $ convox run background-job ./long-running-task.sh --detach --use-service-volume
```

## Version Requirements

- Basic `convox run` functionality: All versions
- GPU support (`--gpu`, `--node-labels`): Requires CLI and rack version >= 3.21.3
- Volume support (`--use-service-volume`): Requires CLI and rack version >= 3.22.3