---
title: "deploy-debug"
slug: deploy-debug
url: /reference/cli/deploy-debug
---
# deploy-debug

Diagnose deploy failures by inspecting an app's Kubernetes pods server-side. This command classifies pods, collects pre-healthcheck logs, gathers Kubernetes events, and maps failure states to actionable hints, all without requiring kubectl or kubeconfig access.

## The Problem

When a deploy fails, `convox logs` often shows nothing because logs are only returned from pods that have passed health checks and reached a ready state. `deploy-debug` closes this visibility gap by querying Kubernetes directly from the rack API.

### Usage
```bash
    convox deploy-debug
```

### Options

| Flag | Short | Description | Default |
|------|-------|-------------|---------|
| `--app` | `-a` | App name | Current directory |
| `--rack` | `-r` | Rack name | Current rack |
| `--output` | `-o` | Output format: `terminal`, `summary`, `json` | `terminal` |
| `--service` | `-s` | Filter to specific service(s), comma-separated | All services |
| `--checks` | `-c` | Run specific checks: `overview`, `init`, `services` | All three |
| `--age` | `-A` | Age threshold in seconds for "new" pod classification | `300` |
| `--lines` | `-n` | Log lines per process | `200` |
| `--all` | | Include healthy processes (normally filtered out) | `false` |
| `--no-events` | | Skip cluster events collection | `false` |
| `--no-previous` | | Skip previous container crash logs | `false` |
| `--watch` | | Re-run on interval (seconds) | Off |

## Diagnostic Checks

The command runs three checks by default. You can select individual checks with `--checks`.

**Overview** (`--checks overview`) lists all Deployments and DaemonSets with their rollout status. Detects stalled deploys (ProgressDeadlineExceeded) and classifies each service as `running`, `deploying`, `stalled`, or `scaled-down`. Shows warning events from the last 30 minutes with actionable hints.

**Init Containers** (`--checks init`) finds pods stuck in init container state and fetches logs from each init container. Reports container state (Running, Waiting, Terminated with exit code).

**Services** (`--checks services`) classifies every pod:
- **unhealthy** - pod phase is not Running
- **not-ready** - Running but health checks are failing
- **new** - Running and ready but age is below threshold (default 300s)
- **healthy** - Running, ready, and established

Collects current and previous container logs, per-pod Kubernetes events, and maps failure states to plain-language hints.

## Failure Hints

The command maps common pod failure states to actionable messages:

| State | Hint |
|-------|------|
| `CrashLoopBackOff` | Process is crash-looping on startup. Check the logs for the error. |
| `ImagePullBackOff` | Failed to pull the container image. Check that the build succeeded and the image tag exists. |
| `OOMKilled` | Process ran out of memory. Increase `scale.memory` in convox.yml. |
| `CreateContainerConfigError` | Container config is invalid. Check environment variables and secrets. |
| `RunContainerError` | Container failed to start. Check the command in convox.yml and that the entrypoint exists. |
| `Unschedulable` | Not enough resources in the cluster. Check `scale.cpu` and `scale.memory` in convox.yml. |
| `ContainersNotReady` | Health check may be failing. Check health check configuration. |
| `PodInitializing` | Init containers may still be running. |

## Examples

Basic usage:
```bash
    $ convox deploy-debug -a myapp
    Deploy Diagnostics: myapp on myrack
    Namespace: myrack-myapp
    Time:      2026-03-18T14:30:00Z

    === Service Overview ===
    SERVICE  DESIRED  READY  STATUS
    web      2        0      deploying

    === Pod Details ===
    POD                           SERVICE  STATUS     HINT
    web-749dd486d8-8v4ss          web      not-ready  Health check may be failing
    web-749dd486d8-k2m9f          web      not-ready  Health check may be failing

    --- web-749dd486d8-8v4ss ---
    Container: main (not-ready)
    Logs:
      Node.js app listening on port 3000
      Error: ECONNREFUSED connecting to database

    Events:
      Unhealthy: Readiness probe failed: HTTP probe failed with statuscode: 503
```

Summary output for quick scanning:
```bash
    $ convox deploy-debug -a myapp -o summary
    SERVICE  DESIRED  READY  STATUS
    web      2        0      deploying

    POD                       STATUS     HINT
    web-749dd486d8-8v4ss      not-ready  Health check may be failing
    web-749dd486d8-k2m9f      not-ready  Health check may be failing
```

JSON output for scripting:
```bash
    $ convox deploy-debug -a myapp -o json | jq '.pods[] | {name: .name, hint: .hint}'
```

Filter to a specific service:
```bash
    $ convox deploy-debug -a myapp -s web
```

Watch mode for ongoing diagnosis:
```bash
    $ convox deploy-debug -a myapp --watch 10
```

## See Also

- [deploy](/reference/cli/deploy) for creating and promoting builds
- [Health Checks](/configuration/health-checks) for configuring readiness and liveness probes
- [Troubleshooting](/help/troubleshooting) for common deployment issues
