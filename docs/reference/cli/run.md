---
title: "run"
description: "The convox run command executes a command in a new one-off process, with flags for CPU, memory, GPU, node placement, pod customization, release, and service volumes."
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
| `--annotations` | string | Pod annotations as comma-separated `key=value` pairs (requires rack >= 3.25.1) |
| `--cpu` | number | CPU request in millicores |
| `--cpu-limit` | number | CPU limit in millicores |
| `--detach` | bool | Run in detached mode |
| `--entrypoint` | string | Override the entrypoint |
| `--gpu` | number | Number of GPUs to allocate (requires rack >= 3.21.3) |
| `--gpu-vendor` | string | GPU vendor for the allocation: `nvidia` (default), `amd`, or `google` for Cloud TPUs on GKE |
| `--id` | bool | With `--detach`, put the process id alone on stdout and all other output on stderr |
| `--labels` | string | Pod labels as comma-separated `key=value` pairs (requires rack >= 3.25.1) |
| `--memory` | number | Memory request in MB |
| `--memory-limit` | number | Memory limit in MB |
| `--node-affinity` | string | Preferred node affinity terms as comma-separated `key=value[:weight]` entries (requires rack >= 3.25.1) |
| `--node-labels` | string | Node labels for targeting specific node groups (requires rack >= 3.21.3) |
| `--release` | string | Run against a specific release |
| `--retain` | number | With `--detach`, seconds to keep the finished process readable by `convox ps info` (requires rack >= 3.25.5) |
| `--termination-grace` | number | Pod terminationGracePeriodSeconds for this run (requires rack >= 3.25.1) |
| `--timeout` | number | Seconds before an attached run is abandoned, or before `--wait` gives up (default `3600`) |
| `--tolerations` | string | Pod tolerations as comma-separated entries (requires rack >= 3.25.1) |
| `--use-service-lifecycle` | bool | Copy the service's lifecycle hooks onto the run container (requires rack >= 3.25.1) |
| `--use-service-volume` | bool | Attach all service-configured volumes to the run pod (requires rack >= 3.22.3) |
| `--wait` | bool | With `--detach`, wait for the process to finish and exit with its status (requires rack >= 3.25.5) |

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

## Detached Runs

`--detach` starts the process and returns as soon as the Rack accepts it.

```bash
    $ convox run web bin/cleanup-database --detach -a my-app
    Running detached process... OK, web-s43xf
      convox logs -a my-app
      convox ps stop web-s43xf -a my-app
```

The two follow-up lines go to standard error. Passing `--wait` suppresses them.

### `--wait`

`--wait` holds the command open until the detached process finishes, then exits with the process's own exit code.

```bash
    $ convox run web bin/migrate --detach --wait -a my-app
    Running detached process... OK, web-s43xf
    $ echo $?
    0
```

The CLI reads the process record every 5 seconds and accepts the result after two consecutive reads report a terminal status, either `complete` or `failed`. `--timeout` bounds the wait, not the process: when the wait runs out, the process keeps running.

`--wait` raises retention to at least 60 seconds so the finished process is still readable when the last read comes back. Pass `--retain` to hold the record longer than that.

`--wait` has no effect without `--detach`. Passing it alone is ignored rather than rejected.

Two conditions end the wait without an exit code, and both exit non-zero. The reads never reached a terminal status:

```text
could not confirm the outcome of process web-s43xf: <reason>
       convox ps info web-s43xf -a my-app
```

The process reached a terminal status carrying no exit code:

```text
process web-s43xf ended with status failed but the rack did not report an exit status.
       It may have been stopped before its command ran, or the rack may predate the release that reports one
```

`-w` is the short form. Every `convox` command carries a `-w` / `--wait` flag kept for compatibility with the V2 CLI, and on every other command it does nothing. On `convox run` it selects the behavior above, so a script that passes `-w` out of habit now waits.

### `--retain`

`--retain <seconds>` keeps a finished detached process readable by `convox ps` and `convox ps info` after its command exits. Without it, the record is removed a few seconds after the process finishes.

```bash
    $ convox run web bin/backfill --detach --retain 600 -a my-app
```

The Rack caps retention at 600 seconds. Values of `0` or less are ignored. `--retain` is accepted only with `--detach`:

```text
--retain is only valid with --detach
```

During the retention window, `convox ps` lists the process with a status of `complete` or `failed`, and `convox ps info` returns it with an `Exit` row.

### `--id`

`--id` puts the process id alone on standard output and moves progress output to standard error, so a shell can capture the id directly.

```bash
    $ pid=$(convox run web bin/backfill --detach --id --retain 600 -a my-app)
    Running detached process... OK, web-s43xf
    $ echo $pid
    web-s43xf
```

`--id` is handled entirely by the CLI and works against any Rack version.

## Detached Runs in CI

### Gate a step on the process

Use `--wait` when the pipeline must stop if the command fails:

```bash
    $ convox run web "bin/migrate" --detach --wait -a my-app
```

The step passes only when the Rack reports an exit code of `0`. It fails when the command exits non-zero, when the wait exceeds `--timeout`, and when no exit status arrives.

### Start now, check later

Use `--id` and `--retain` when the pipeline starts the work in one step and inspects it in a later one:

```bash
    $ pid=$(convox run web "bin/backfill" --detach --id --retain 600 -a my-app)
```

Then, within the retention window:

```bash
    $ convox ps info "$pid" -a my-app
    Id        web-s43xf
    App       my-app
    Command   bin/backfill
    Instance  i-0cbaa6d2dd1d094c0
    Release   RCRLBREFPBX
    Service   web
    Started   4 minutes ago
    Status    complete
    Exit      0
```

Retention is capped at 600 seconds, and `--wait` raises it to 60 seconds when the run asks for less. Retention is best effort: a Rack API restart or a node replacement can drop the record before its window ends. A process the pipeline can no longer find is an unknown outcome, and the step must treat it as a failure rather than as success.

## Exit Status

`convox run` exits with the command's exit code. When the command's output stream ends without one, the CLI prints an error and exits `1`:

```text
the rack did not report an exit status for this command, so it may not have finished.
       Check the output above for a reason. This run's process has been stopped.
       A command that must gate a deploy should write its own success marker to the output for the caller to check
```

Earlier CLI versions exited `0` here, so a CI step gated on `convox run` passed while the command's outcome was unknown.

A stream lost in transit is caught by the CLI on its own and needs no Rack upgrade. An error the Rack raises before the command starts needs Rack 3.25.5 or later; earlier Racks send `0` as the exit status even when the command never ran.

On an attached run, if your connection ends before the command finishes, the command keeps running in its pod with no output and no exit status. On this path `--timeout` bounds the pod itself, not only the wait: the pod ends when the timeout elapses, one hour by default, and takes the command with it. That bound applies when the CLI is gone or can no longer reach the Rack; a CLI that survives a broken stream asks the Rack to stop the process as it exits, which removes the run pod right away. Use `--detach` for work that must be tracked rather than abandoned.

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

This works with custom node group configurations. For example, if you've set up GPU nodes:

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

Then `convox run gpu-worker bash` automatically runs on the `gpu` pool, with no `--node-labels` flag needed.

### Override with `--node-labels`

To send a run pod to a different node pool (for example, to debug a GPU service on general-purpose nodes):

```bash
    $ convox run gpu-worker bash --node-labels "convox.io/nodepool=workload"
```

This clears the inherited placement and applies the specified labels instead.

On a Rack running Karpenter at `3.25.4` or later, a `convox.io/nodepool` value the Rack does not have is rejected before the pod is created, and the error lists the pools that do exist. See [Node Pool Validation](/configuration/scaling/karpenter#node-pool-validation).

### Clear inherited node placement

To remove the inherited node affinity and allow the pod to schedule on general cluster nodes:

```bash
    $ convox run gpu-worker bash --node-labels ""
```

This is useful for debugging when you want to run a one-off process outside its usual dedicated pool.

> Builds are not affected by automatic node placement. `convox build` always uses the configured build nodes regardless of `nodeSelectorLabels`.

## Pod Customization Flags

Six flags customize the one-off process pod for a single invocation without changing `convox.yml` or the deployed Service. All are optional; runs that do not pass them behave exactly as before. On racks 3.25.1 and later, input is validated on the Rack and malformed entries return an error before any pod is created; older racks ignore the flags.

### `--termination-grace`

Sets the pod's `terminationGracePeriodSeconds` for this run. Must be `0` or greater. When not passed, the run pod uses the service's `termination.grace` setting from `convox.yml` (`30` if unset).

```bash
    $ convox run web bin/migrate --termination-grace 900
```

### `--annotations` and `--labels`

Add pod annotations or labels as comma-separated `key=value` pairs.

```bash
    $ convox run web bin/backfill --annotations karpenter.sh/do-not-disrupt=true --labels purpose=batch
```

- Annotation keys already set by Convox are left unchanged.
- Convox-reserved label keys (`app`, `rack`, `service`, `system`, `type`, `name`, `release`, `service-type`) are rejected.
- On Karpenter racks, `--annotations karpenter.sh/do-not-disrupt=true` keeps the run pod's node from being voluntarily consolidated until the process finishes.

### `--use-service-lifecycle`

Copies the service's `lifecycle.postStart` and `preStop` hooks from `convox.yml` onto the run container, so one-off processes can perform the same setup and teardown as their parent service.

```bash
    $ convox run web bin/task --use-service-lifecycle
```

### `--node-affinity`

Adds preferred node affinity terms as comma-separated `key=value:weight` entries. Weight is optional, must be between `1` and `100`, and defaults to `100`. The affinity is a preference, not a requirement; the pod still schedules elsewhere if no matching node is available.

```bash
    $ convox run web bin/backfill --node-affinity workload=batch:80
```

### `--tolerations`

Adds pod tolerations as comma-separated entries in `key=value:Effect`, `key:Effect`, `key=value`, or bare `key` form. Valid effects are `NoSchedule`, `PreferNoSchedule`, and `NoExecute`; omitting the effect matches taints with any effect.

```bash
    $ convox run web bin/backfill --tolerations dedicated=batch:NoSchedule
```

### Combining with `--node-labels`

`--node-affinity` and `--tolerations` work together with `--node-labels`: when combined, `--node-labels` resets service-inherited affinity and tolerations first, and entries from `--node-affinity` and `--tolerations` are applied after that reset.

```bash
    $ convox run web bin/backfill --node-labels "convox.io/nodepool=batch" --node-affinity zone=us-east-1a:80 --tolerations dedicated=batch:NoSchedule
```

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
- Pod customization flags (`--termination-grace`, `--annotations`, `--labels`, `--use-service-lifecycle`, `--node-affinity`, `--tolerations`): Requires CLI and rack version >= 3.25.1. Against older racks the flags are ignored
- Node pool validation on `--node-labels`: Requires rack version >= 3.25.4. The check runs on the rack, so it applies to any CLI version
- Detached wait and retention (`--wait`, `--retain`): Requires CLI and rack version >= 3.25.5
- `--id`: Handled by the CLI, so it applies to any rack version
- Failing on a missing exit status: The CLI catches a stream lost in transit on its own; an error the rack raises before the command starts requires rack version >= 3.25.5

## See Also

- [One-off Commands](/management/run) for run command patterns
- [Workload Placement](/configuration/scaling/workload-placement) for `nodeSelectorLabels` configuration
- [ps](/reference/cli/ps) for inspecting a detached or retained process
- [Karpenter](/configuration/scaling/karpenter) for dedicated pool isolation with `dedicated: true`