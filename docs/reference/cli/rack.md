---
title: "rack"
slug: rack
url: /reference/cli/rack
---
# rack

## Command Summary

| Command | Description |
|---------|-------------|
| **Information** | |
| `convox rack` | Display rack info |
| `convox rack ps` | List rack processes |
| `convox rack ps --all` | List all rack processes including system |
| `convox rack logs` | Get logs for the rack |
| **Installation** | |
| `convox rack install` | Install a new rack |
| `convox rack uninstall` | Uninstall a rack |
| `convox rack update` | Update rack to a new version |
| **Access** | |
| `convox rack access` | Generate rack access credential |
| `convox rack access key rotate` | Rotate rack access key |
| **Runtime** | |
| `convox rack runtimes` | List attachable runtime integrations |
| `convox rack runtime attach` | Attach a runtime integration |
| **Parameters** | |
| `convox rack params` | Display rack parameters |
| `convox rack params set` | Set rack parameters |
| **Scaling** | |
| `convox rack releases` | List rack version history |
| `convox rack scale` | Scale the rack |
| **Karpenter** | |
| `convox rack karpenter cleanup` | Clean up orphaned Karpenter nodes after disabling |
| **Kubernetes** | |
| `convox rack kubeconfig` | Output kubeconfig for the underlying cluster |
| `convox rack mv` | Transfer rack management between user and org |

## rack

Get information about the rack

### Usage
```bash
    convox rack
```
### Examples
```bash
    $ convox rack
    Name      test
    Provider  aws
    Router    router.0a1b2c3d4e5f.convox.cloud
    Status    running
    Version   3.23.3
```
## rack install

Install a new Rack

> note: To install the rack into the console with the specified runtime, region, and optional [parameters](/configuration/rack-parameters): provide orgname/rackname in place of `<name>`

### Usage
```bash
    convox rack install <provider> <name> [option=value]...

    convox rack install <provider> <orgname>/<rackname> region=<region> --runtime=<runtime-id> [option=value]...
```

flags:
  - `runtime`: runtime integration ID
  - `version`: specify the rack version to install

> note: To install a rack into an organization with runtime integration, ensure your CLI is updated to the latest version. For detailed instructions on updating CLI, see [CLI Management](/management/cli-rack-management).

> note: Obtain the runtime ID by running `convox runtimes <orgname>`.

### Examples
```bash
    $ convox rack install local dev
    ...

    $ convox rack install aws production region=eu-west-1 node_type=t3.large
    ...

    $ convox rack install aws my-org/staging region=us-east-1 --runtime=20e58437-fab7-4124-aa5a-2e5978f1149e
    ...
```

## rack kubeconfig

Output a Kubernetes configuration file for connecting to the underlying cluster

### Usage
```bash
    convox rack kubeconfig
```
### Examples
```bash
    $ convox rack kubeconfig
    apiVersion: v1
    clusters:
    - cluster:
        server: https://api.0a1b2c3d4e5f.convox.cloud/kubernetes/
    name: kubernetes
    contexts:
    - context:
        cluster: kubernetes
        user: proxy
    name: proxy@kubernetes
    current-context: proxy@kubernetes
    kind: Config
    preferences: {}
    users:
    - name: proxy
    user:
        username: convox
        password: abcdefghijklmnopqrstuvwxyz
```
## rack logs

Get logs for the rack

### Usage
```bash
    convox rack logs
```
### Examples
```bash
    $ convox rack logs
    2026-01-15T13:37:22Z service/web/a55eb25e-90f5-4301-99fd-e35c91128592 ns=provider.aws at=SystemGet state=success elapsed=275.683
    2026-01-15T13:37:22Z service/web/a55eb25e-90f5-4301-99fd-e35c91128592 id=8d3ec85dc324 ns=api at=SystemGet method="GET" path="/system" response=200 elapsed=276.086
    2026-01-15T13:38:04Z service/web/a55eb25e-90f5-4301-99fd-e35c91128592 ns=provider.aws at=SystemGet state=success elapsed=331.824
    2026-01-15T13:38:04Z service/web/a55eb25e-90f5-4301-99fd-e35c91128592 id=f492a0dce931 ns=api at=SystemGet method="GET" path="/system" response=200 elapsed=332.219
    ...
```
## rack mv

Transfer the management of a Rack from an individual user to an organization or vice versa.
> note: rack name should be same

### Usage
```bash
    convox rack mv <from> <to>
```
### Examples
```bash
    $ convox rack mv dev acme/dev
    moving rack dev to acme/dev... OK

    $ convox rack mv acme/dev dev
    moving rack acme/dev to dev... OK
```
## rack ps

List rack processes

### Usage
```bash
    convox rack ps
```
### Examples
```bash
    $ convox rack ps
    ID                       APP     SERVICE        STATUS   RELEASE       STARTED      COMMAND
    api-9749b7ccb-29zh5      system  api            running  3.23.3        2 weeks ago  api
    api-9749b7ccb-29zh5      rack    api            running  3.23.3        2 weeks ago  api
    api-9749b7ccb-cg4hr      system  api            running  3.23.3        2 weeks ago  api
    api-9749b7ccb-cg4hr      rack    api            running  3.23.3        2 weeks ago  api
    atom-578cd48bfb-6tm7g    rack    atom           running  3.23.3        2 weeks ago  atom
    atom-578cd48bfb-6tm7g    system  atom           running  3.23.3        2 weeks ago  atom
    router-846b84d544-ndz76  rack    router         running  3.23.3        2 weeks ago  router
    router-846b84d544-ndz76  system  router         running  3.23.3        2 weeks ago  router
```
## rack ps --all

List rack processes as well as essential system ones running on the Rack

### Usage
```bash
    convox rack ps --all
```
### Examples
```bash
    $ convox rack ps --all
    ID                       APP     SERVICE        STATUS   RELEASE       STARTED      COMMAND
    api-9749b7ccb-29zh5      system  api            running  3.23.3        2 weeks ago  api
    api-9749b7ccb-29zh5      rack    api            running  3.23.3        2 weeks ago  api
    api-9749b7ccb-cg4hr      system  api            running  3.23.3        2 weeks ago  api
    api-9749b7ccb-cg4hr      rack    api            running  3.23.3        2 weeks ago  api
    atom-578cd48bfb-6tm7g    rack    atom           running  3.23.3        2 weeks ago  atom
    atom-578cd48bfb-6tm7g    system  atom           running  3.23.3        2 weeks ago  atom
    elasticsearch-0          rack    elasticsearch  running  3.23.3        2 weeks ago
    elasticsearch-0          system  elasticsearch  running  3.23.3        2 weeks ago
    elasticsearch-1          rack    elasticsearch  running  3.23.3        2 weeks ago
    elasticsearch-1          system  elasticsearch  running  3.23.3        2 weeks ago
    fluentd-p56dk            rack    fluentd        running  3.23.3        2 weeks ago
    fluentd-p56dk            system  fluentd        running  3.23.3        2 weeks ago
    fluentd-qrttw            rack    fluentd        running  3.23.3        2 weeks ago
    fluentd-qrttw            system  fluentd        running  3.23.3        2 weeks ago
    fluentd-zsv8f            rack    fluentd        running  3.23.3        2 weeks ago
    fluentd-zsv8f            system  fluentd        running  3.23.3        2 weeks ago
    redis-77b4f65c55-nbx89   rack    redis          running  3.23.3        2 weeks ago
    redis-77b4f65c55-nbx89   system  redis          running  3.23.3        2 weeks ago
    router-846b84d544-ndz76  rack    router         running  3.23.3        2 weeks ago  router
    router-846b84d544-ndz76  system  router         running  3.23.3        2 weeks ago  router
```

## rack runtimes

List of attachable runtime integrations

### Usage
```bash
    convox rack runtimes
```
### Examples
```bash
    $ convox rack runtimes
    ID                                    TITLE
    20e58437-fab7-4124-aa5a-2e5978f1149e  047979207916
```

## rack runtime attach

Attach runtime integration to the rack

### Usage
```bash
    convox rack runtime attach <runtime_id>
```
### Examples
```bash
    $ convox rack runtime attach 20e58437-fab7-4124-aa5a-2e5978f11
    OK
```

## rack uninstall

Uninstalls a Rack

### Usage
```bash
    convox rack uninstall <name>
```
### Examples
```bash
    $ convox rack uninstall my-rack
    Upgrading modules...
    Downloading github.com/convox/convox?ref=3.23.3 for system...
    ...
    Destroy complete! Resources: 35 destroyed.
```
## rack update

Updates a Rack to a new version.

### Usage
```bash
    convox rack update [version]
```
### Examples
```bash
    $ convox rack update
    Upgrading modules...
    Downloading github.com/convox/convox?ref=3.23.3 for system...
    ...
    Apply complete! Resources: 1 added, 4 changed, 1 destroyed.

    Outputs:

    api = https://convox:password@api.dev.convox
    provider = local
    OK
```

## rack params

Display rack parameters

### Usage
```bash
    convox rack params [--group <name>] [--reveal]
```

### Flags

| Flag | Short | Description |
|------|-------|-------------|
| `--group` | `-g` | Filter output to a curated logical group. Accepts exact names and unique prefixes |
| `--reveal` | | Show unmasked values for sensitive parameters on a TTY |

### Examples
```bash
    $ convox rack params
    build_node_enabled     false
    high_availability      true
    node_disk              20
    node_type              t3.small
    private                true
```

### Masking Sensitive Values

Values for `docker_hub_password`, `secret_key`, `token`, `access_id`, `private_eks_host`, `private_eks_user`, and `private_eks_pass` render as `**********` on a TTY to prevent accidental disclosure via terminal output, shell history, screen shares, or captured logs. Empty values display as empty so you can tell whether a sensitive parameter is set. The stored values are unchanged.

Piped and non-TTY output is always unmasked, so backup and automation flows (`convox rack params > rack-state.txt`, `| grep`, `| jq`) continue to return real values. Use `--reveal` on a TTY for one-off inspection when the real value is needed.

```bash
    $ convox rack params | grep docker_hub_password    # pipe: real value
    docker_hub_password    s3cret-t0ken

    $ convox rack params                                # TTY: masked
    docker_hub_password    **********

    $ convox rack params --reveal                       # TTY override: real value
    docker_hub_password    s3cret-t0ken
```

### Group Filter

Pass `-g` / `--group` to narrow the output to one of a curated set of logical groups. Useful when a fully featured rack emits 100+ parameters and you want the handful relevant to a specific task.

| Group | Covers |
|-------|--------|
| `build` | build node config, buildkit, additional build groups |
| `domain` | rack domain and TLS toggle |
| `ingress` | NGINX, idle timeout, TLS cert duration |
| `karpenter` | Karpenter autoscaling configuration |
| `logging` | syslog, telemetry, fluentd |
| `network` | VPC, subnets, CIDR, routing, NLB, DNS resolver |
| `nodes` | default node-group config, user-data, GPU, kubelet tuning |
| `registry` | Docker Hub, ECR, image caching, storage buckets |
| `retention` | release retention policy |
| `scaling` | capacity counts, HA, HPA/VPA/KEDA, schedules, PDB, disruption budgets |
| `security` | access controls, whitelist, IAM, encryption, private EKS, IMDS, TLS, credentials |
| `storage` | CSI drivers, EBS/EFS/Azure Files, registry disk |
| `versions` | K8s and managed component versions |

Exact names and unique prefixes both resolve:

```bash
    $ convox rack params -g karpenter -r prod
    karpenter_arch                    amd64
    karpenter_auth_mode               true
    karpenter_enabled                 true
    ...

    $ convox rack params -g karp -r prod              # prefix match
    ...

    $ convox rack params --group security -r prod     # long form
    access_id                          **********
    disable_public_access              false
    docker_hub_password                **********
    private_eks_host                   **********
    ...
```

Ambiguous prefixes print a disambiguating hint and the full group list so names are discoverable without leaving the command:

```bash
    $ convox rack params -g s
    ERROR: group 's' matches multiple groups: scaling, security, storage (use 'sca', 'sec', or 'sto')
      available groups:
      build        build node config, buildkit, additional build groups
      domain       rack domain and TLS toggle
      ...
```

Unknown groups print the same list. The filter is applied client-side on top of the rack response, so sensitive-value masking and `--reveal` compose with `--group` (`convox rack params -g security --reveal` shows unmasked credentials).

## rack params set

Set rack parameters

### Usage
```bash
    convox rack params set <Key=Value> [Key=Value]... [--force]
```

### Flags

| Flag | Short | Description |
|------|-------|-------------|
| `--force` | `-f` | Override unknown-key and managed-parameter guards |

### Examples
```bash
    $ convox rack params set node_type=t3.large
    Setting parameters... OK
```

### Parameter Validation

The CLI validates parameters before sending them to the Rack. This catches common errors before they reach Terraform:

- **Unknown parameters** are rejected with a suggestion if a close match exists:
  ```
  unknown parameter 'node_tyep' for aws provider
         Did you mean 'node_type'?
         Run 'sudo convox update' to get the latest parameter support, or use --force to override.
  ```
- **Install-only parameters** (`high_availability`, `private`, `cidr`, `vpc_id`, `region`, `availability_zones`, subnet IDs) cannot be changed after rack creation.
- **Managed parameters** (`image`, `release`, `k8s_version`, addon versions) are set automatically by `convox rack update` and cannot be modified directly.
- **Empty values** are rejected for parameters that require explicit values. Parameters that support clearing (labels, taints, schedules, credentials) accept empty strings.
- **Type validation** is enforced for specific parameters (e.g., `karpenter_auth_mode` must be `true` or `false`, `node_capacity_type` must be `on_demand`, `spot`, or `mixed`).

Use `--force` to bypass the unknown-key and managed-parameter guards. Install-only, empty-value, and type validators cannot be overridden.

> The `schedule_rack_scale_down` and `schedule_rack_scale_up` parameters must be set together.

## rack karpenter cleanup

Clean up orphaned Karpenter nodes after disabling Karpenter. Terminates backing EC2 instances, cordons and drains Kubernetes nodes (respecting PodDisruptionBudgets), strips Karpenter finalizers, and removes stale CRD objects (NodePools, NodeClaims, EC2NodeClasses).

Safe to run multiple times. See [Disabling Karpenter](/configuration/scaling/karpenter#disabling-karpenter) for the full workflow.

### Usage
```bash
    convox rack karpenter cleanup
```

### Examples
```bash
    $ convox rack karpenter cleanup -r rackName
    Cleaning up Karpenter nodes... OK
```

## rack releases

List rack version history

### Usage
```bash
    convox rack releases
```
### Examples
```bash
    $ convox rack releases
    VERSION  UPDATED
    3.23.4   2 days ago
    3.23.3   2 weeks ago
    3.23.2   3 weeks ago
```

## rack scale

Scale the rack

### Usage
```bash
    convox rack scale
```

### Flags

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--count` | `-c` | int | Instance count |
| `--type` | `-t` | string | Instance type |

### Examples
```bash
    $ convox rack scale
    Autoscale  Yes
    Count      3
    Status     running
    Type       t3.small

    $ convox rack scale --count 5 --type t3.large
    Scaling rack... OK
```

## rack access credential

Generates rack access credential

### Usage
```bash
    convox rack access --role [role] --duration-in-hours [duration]
```
flags:
  - `role`: Access role for the credential. Allowed roles are: `read` or `write`
  - `duration-in-hours`: TTL for the credential.

### Examples
```bash
    $ convox rack access --role read --duration-in-hours 1
    RACK_URL=https://...

    $ export RACK_URL=https://...
    $ convox rack
    Name      v3-rack
    Provider  aws
    Router    router.convox
    Status    running
    Version   ...

```

## rack access key rotation

Rotates the rack access key that is used for rack access credential. It will invalidate previous all the credential generated from ` convox rack access --role [role] --duration-in-hours [duration]`.

### Usage
```bash
    convox rack access key rotate
```

### Examples
```bash
    $ convox rack access key rotate
    OK

```

## See Also

- [Rack Parameters](/configuration/rack-parameters) for parameter reference
- [CLI Rack Management](/management/cli-rack-management) for management best practices