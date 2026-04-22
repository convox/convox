---
title: "Service"
slug: service
url: /reference/primitives/app/service
---
# Service

A Service is a horizontally-scalable collection of durable [Processes](/reference/primitives/app/process).

[Processes](/reference/primitives/app/process) that belong to a Service are automatically restarted upon termination.

Services can be scaled to a static count or autoscaled in a range based on metrics.

## Service Definition

```yaml
services:
  web:
    build: .
    health: /check
    port: 5000
    scale: 3
```

```yaml
services:
  grpc:
    build: .
    port: grpc:5551
    grpcHealthEnabled: true
```

```yaml
services:
  web:
    agent: false
    annotations:
      - test.annotation.org/value=foobar
    build:
      manifest: Dockerfile
      path: .
    certificate:
      duration: 2160h
    command: bin/web
    deployment:
      minimum: 25
      maximum: 100
    domain: ${WEB_HOST}
    dnsConfig:
      ndots: 5
    environment:
      - FOO
      - BAR=qux
    health:
      grace: 10
      interval: 5
      path: /check
      timeout: 3
    liveness:
      path: /liveness/check
      grace: 15
      interval: 5
      timeout: 3
      successThreshold: 1
      failureThreshold: 3
    internal: false
    initContainer:
      command: "bin/migrate"
    ingressAnnotations:
      - nginx.ingress.kubernetes.io/limit-rpm=10
    labels:
      convox.com/test: true
    lifecycle:
      preStop: "sleep 10"
      postStart: "sleep 10"
    port: 5000
    ports:
      - 5001
      - port: 5002
        protocol: udp
    privileged: false
    scale:
      count: 1-3
      limit:
        cpu: 500
        memory: 1024
      cpu: 250
      memory: 512
      targets:
        cpu: 50
        memory: 80
        external:
          - name: "datadogmetric@default:web-requests"
            averageValue: 200
    securityContext:
      runAsNonRoot: true
      readOnlyRootFilesystem: true
      allowPrivilegeEscalation: false
      capabilities:
        drop:
          - ALL
      seccompProfile: RuntimeDefault
    singleton: false
    sticky: true
    termination:
      grace: 45
    test: make test
    timeout: 180
    tls:
      redirect: true
    whitelist: 10.0.0.128/16,192.168.0.1/32
```

| Attribute     | Type       | Default             | Description                                                                                                                                |
| ------------- | ---------- | ------------------- | ------------------------------------------------------------------------------------------------------------------------------------------ |
| **agent**       | boolean    | false               | Set to **true** to declare this Service as an [Agent](/configuration/agents)                                                      |
| **annotations** | list       |                     | A list of annotation keys and values to populate the metadata for the deployed pods and their serviceaccounts                              |
| **accessControl** | map       |                     | Specification of the pod access control management. Currently only IAM using AWS pod identity is supported |
| **build**       | string/map | .                   | Build definition (see below)                                                                                                                                            |
| **certificate**| map         |                     | Define certificate parameters                                                                       |
| **command**     | string     | **CMD** of Dockerfile | The command to run to start a [Process](/reference/primitives/app/process) for this Service                                                                       |
| **deployment**  | map        |                     | Manual control over deployment parameters                                                                                                  |
| **domain**      | string     |                     | A custom domain(s) (comma separated) to route to this Service                                                                              |
| **dnsConfig**      | map     |                     | DNS configuration for the service |
| **environment** | list       |                     | A list of environment variables (with optional defaults) to populate from the [Release](/reference/primitives/app/release) environment                            |
| **grpcHealthEnabled** | boolean   |      false          | Enables gRPC health checking (configures both readiness and liveness probes). Must follow the [gRPC health protocol](https://github.com/grpc/grpc/blob/master/doc/health-checking.md). See [Health Checks](/configuration/health-checks#grpc-health-checks). |
| **health**      | string/map | /                   | Health check definition (see below)                                                                                                        |
| **liveness** | map |      | Liveness check definition (see below). By default it is disabled. If it fails then service will restart |
| **startupProbe** | map |  | Startup probe definition. Set `path` (HTTP) or `tcpSocketPort` (TCP) to enable. All timing parameters are inherited from the **liveness** check; setting timing fields directly on `startupProbe` has no effect. See [Health Checks](/configuration/health-checks#startup-probes) |
| **image**       | string     |                     | An external Docker image to use for this Service (supersedes **build**)                                                                      |
| **ingressAnnotations** | list       |                     | A list of annotation keys and values to add in ingress resource. Check below for reserved annotation keys |
| **initContainer** | map       |                     | Runs a container to completion before the main service container starts. Use for migrations, dependency checks, or setup tasks (see [initContainer](#initcontainer) below) |
| **internal**    | boolean    | false               | Set to **true** to make this Service only accessible inside the Rack                                                                         |
| **internalRouter** | boolean    | false               | Set to **true** to make this Service only accessible using the internal load balancer. Requires the rack parameter [internal_router](/configuration/rack-parameters/aws/internal_router) to also be **true**. |
| **init**        | boolean    | true                | Set to **false** to disable the init process for this Service. When enabled, an init process runs as PID 1 and handles signal forwarding and zombie process reaping |
| **labels** |  map  |       | Custom labels for k8s resources. See here for [syntax and character set](https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/#syntax-and-character-set). The following keys are reserved: `system`, `rack`, `app`, `name`, `service`, `release`, `type` |
| **lifecycle** |  map  |       | The prestop and poststart hooks enable running commands before terminating and after starting the container, respectively |
| **configMounts** | list |     | Mount configuration files into the container filesystem. See [Config Mounts](/configuration/config-mounts) |
| **nodeAffinityLabels** | map |  | Node affinity rules for workload placement. See [Workload Placement](/configuration/scaling/workload-placement) |
| **nodeSelectorLabels** | map |  | Node selector labels for workload placement. See [Workload Placement](/configuration/scaling/workload-placement) |
| **port**        | string     |                     | The port that the default Rack balancer will use to [route incoming traffic](/configuration/load-balancers). For grpc service specify the scheme: `grpc:5051`|
| **ports**       | list       |                     | A list of ports available for internal [service discovery](/configuration/service-discovery) or custom [Balancers](/reference/primitives/app/balancer). Supports TCP (default) and UDP protocols |
| **privileged**  | boolean    | false               | Set to **true** to allow [Processes](/reference/primitives/app/process) of this Service to run as root inside their container. Use with caution as this grants elevated permissions |
| **resources**   | list       |                     | A list of [Resources](/reference/primitives/app/resource) to make available to this Service (e.g. databases) |
| **scale**       | map        | 1                   | Define scaling parameters (see below)                                                                                                      |
| **securityContext** | map   |                     | Container security settings including capabilities, read-only filesystem, and seccomp profiles (see below)                               |
| **singleton**   | boolean    | false               | Set to **true** to prevent extra [Processes](/reference/primitives/app/process) of this Service from being started during deployments                               |
| **sticky**      | boolean    | false               | Set to **true** to enable sticky sessions                                                                                                    |
| **termination** | map        |                     | Termination related configuration                                                                                                          |
| **test**        | string     |                     | A command to run to test this Service when running **convox test**                                                                           |
| **timeout**     | number     | 60                  | Timeout period (in seconds) for reading/writing requests to/from your service                                                              |
| **tls**         | map        |                     | TLS-related configuration                                                                                                                  |
| **volumes**        | list    |                     | List of [Volumes](/configuration/volumes) to mount (short-form string syntax, e.g. `/data`) |
| **volumeOptions**  | list    |                     | List of volumes to attach with service (advanced configuration, see below) |
| **whitelist**   | string     |                     | Comma delimited list of CIDRs, e.g. `10.0.0.0/24,172.10.0.1`, to allow access to the service                                                                                                                  |

> Environment variables declared on `convox.yml` will be populated for a Service.

> The `drain` attribute is deprecated. Use `termination.grace` instead.

### annotations
You can use annotations to attach arbitrary non-identifying metadata to objects. Clients such as tools and libraries can retrieve this metadata. On Convox, annotations will reflect in pods and service accounts.

Convox also recognizes `convox.com/pdb-disabled=true` as a way to opt a service out of its Convox-managed PodDisruptionBudget. See [Disabling PDB for a Service](/configuration/scaling/autoscaling#disabling-pdb-for-a-service) for details.

Here are some examples of information that can be recorded in annotations:
- Build, release, or image information like timestamps, release IDs, git branch, PR numbers, image hashes, and registry address.
- Fields managed by a declarative configuration layer. Attaching these fields as annotations distinguishes them from default values set by clients or servers, and from auto-generated fields and fields set by auto-sizing or auto-scaling systems.
- User or tool/system provenance information, such as URLs of related objects from other ecosystem components.
- Configure a service to assume an AWS IAM Role via IRSA. For example:

```yaml
environment:
  - PORT=3000
services:
  web:
    annotations:
      - eks.amazonaws.com/role-arn=arn:aws:iam::accountID:role/yourOwnIAMRole
    domain: ${HOST}
    build: .
    port: 3000
```
### accessControl

| Attribute  | Type   | Default    | Description                                                   |
| ---------- | ------ | ---------- | ------------------------------------------------------------- |
| **awsPodIdentity** | map |  | The specification for IAM Role for AWS Pod Identity. This will only work if pod identity is enable on the rack. |

```yaml
services:
  web:
    build: .
    port: 3000
    accessControl:
      awsPodIdentity:
        policyArns:
         - "arn:aws:iam::aws:policy/AmazonEC2ContainerRegistryReadOnly"
         - "arn:aws:iam::aws:policy/AmazonS3ReadOnlyAccess"
```

### accessControl.awsPodIdentity

| Attribute  | Type   | Default    | Description                                                   |
| ---------- | ------ | ---------- | ------------------------------------------------------------- |
| **policyArns** | list |  | The list of policy ARNs for the IAM role |

> Pod identity must be enabled on rack before specifying this.

### build

| Attribute  | Type   | Default    | Description                                                   |
| ---------- | ------ | ---------- | ------------------------------------------------------------- |
| **args**     | list   |            | Build arguments to pass during the build. See [Build Arguments](/reference/primitives/app/build#build-arguments) |
| **manifest** | string | Dockerfile | The filename of the Dockerfile                                |
| **path**     | string | .          | The path (relative to **convox.yml**) to build for this Service |

> Specifying **build** as a string will set the **path** and leave the other values as defaults.

### certificate

| Attribute  | Type   | Default    | Description                                                   |
| ---------- | ------ | ---------- | ------------------------------------------------------------- |
| **duration** | string | 2160h | Certificate renew frequency period                                |
| **id** | string |  | ID of the generated certificate to use instead of creating a new certificate. If specified, the `duration` value will not have any effect, since the certificate is already generated |

### deployment

| Attribute | Type   | Default | Description                                                                      |
| --------- | ------ | ------- | -------------------------------------------------------------------------------- |
| **maximum** | number | 200     | The maximum percentage of Processes to allow during rolling deploys. Defaults to 100 for agents and singletons. |
| **minimum** | number | 50      | The minimum percentage of healthy Processes to keep alive during rolling deploys. Defaults to 0 for agents and singletons. |



### dnsConfig

| Attribute | Type   | Default | Description                                                                                |
| --------- | ------ | ------- | ------------------------------------------------------------------------------------------ |
| **ndots**     | int |         | The ndots option for the dns config |



### ingressAnnotations

This accepts list of strings where in each string annotation key and value is separated by `=` sign:

```yaml
services:
  web:
    build: .
    port: 3000
    ingressAnnotations:
      - nginx.ingress.kubernetes.io/limit-rpm=10
      - nginx.ingress.kubernetes.io/enable-access-log=false
```

Reserved annotation keys:
- `alb.ingress.kubernetes.io/scheme`
- `cert-manager.io/cluster-issuer`
- `cert-manager.io/duration`
- `nginx.ingress.kubernetes.io/backend-protocol`
- `nginx.ingress.kubernetes.io/proxy-connect-timeout`
- `nginx.ingress.kubernetes.io/proxy-read-timeout`
- `nginx.ingress.kubernetes.io/proxy-send-timeout`
- `nginx.ingress.kubernetes.io/server-snippet`
- `nginx.ingress.kubernetes.io/affinity`
- `nginx.ingress.kubernetes.io/session-cookie-name`
- `nginx.ingress.kubernetes.io/ssl-redirect`
- `nginx.ingress.kubernetes.io/whitelist-source-range`



### initContainer

An init container runs to completion before the main service container starts. Use init containers to perform setup tasks such as database migrations, waiting for upstream dependencies, or preparing configuration. If the init container exits with a non-zero status, the pod will restart and retry it.

The init container automatically receives:
- All service [environment variables](/configuration/environment)
- All service [resource](/reference/primitives/app/resource) connections (e.g. `DATABASE_URL`)
- An `INIT_CONTAINER=true` environment variable

| Attribute | Type   | Default | Description                                                                                |
| --------- | ------ | ------- | ------------------------------------------------------------------------------------------ |
| **image**     | string | service image | An external Docker image to run. If not set, uses the service image |
| **command**  | string |         | The command to run. **Required** for the init container to be created |
| **configMounts** | list |      | Mount configuration files into the init container. See [Config Mounts](/configuration/config-mounts) |
| **volumeOptions**  | list |         | List of volumes to attach (see [volumeOptions](#volumeoptions)) |

#### Database Migrations

Run migrations before the application starts:

```yaml
services:
  web:
    build: .
    port: 3000
    resources:
      - database
    initContainer:
      command: "bin/migrate"
```

The init container connects to the database using the same resource credentials as the main service.

#### Waiting for Dependencies

Use a lightweight image to block startup until upstream services are ready:

```yaml
services:
  api:
    build: .
    port: 3000
    initContainer:
      image: busybox:1.36
      command: |
        sh -c '
        echo "Waiting for service-a...";
        until wget -qO /dev/null http://service-a:8080/healthz 2>/dev/null; do sleep 5; done;
        echo "service-a ready";
        echo "Waiting for service-b...";
        until wget -qO /dev/null http://service-b:9000/health 2>/dev/null; do sleep 5; done;
        echo "service-b ready";
        echo "All dependencies ready!"'
```

This ensures the main container only starts after its dependencies are healthy. Specifying a minimal `image` like `busybox` avoids building dependency-checking logic into your application image.



### lifecycle

| Attribute | Type   | Default | Description                                                                                |
| --------- | ------ | ------- | ------------------------------------------------------------------------------------------ |
| **preStop**     | string |         | Command to run before the container is terminated |
| **postStart**  | string |         | Command to run immediately after the container starts |

Use lifecycle hooks to manage graceful shutdown and post-start initialization:

```yaml
services:
  web:
    build: .
    port: 3000
    lifecycle:
      preStop: "sleep 10"
      postStart: "/bin/sh -c 'echo started > /tmp/ready'"
```

The `preStop` hook runs before a container receives the SIGTERM signal during shutdown. A common pattern is to add a brief sleep to allow in-flight requests to complete and load balancers to deregister the pod:

```yaml
lifecycle:
  preStop: "sleep 10"
```

The `postStart` hook runs immediately after the container starts, in parallel with the main process. Use it for tasks like cache warming or service registration.

See [Health Checks](/configuration/health-checks) for configuring readiness, liveness, and startup probes that work alongside lifecycle hooks.



### health

| Attribute  | Type   | Default | Description                                                                                      |
| ---------- | ------ | ------- | ------------------------------------------------------------------------------------------------ |
| **grace**    | number | `interval` | The number of seconds to wait for a [Process](/reference/primitives/app/process) to start before starting health checks. Defaults to the value of `interval` |
| **interval** | number | 5       | The number of seconds between health checks                                                      |
| **path**     | string | /       | The path to request for health checks                                                            |
| **port**     | number or map | Main service port | The port the readiness probe connects to. Accepts a scalar (`port: 8080`) or a map (`port: { port: 8080, scheme: https }`). Scheme inherits from the main service port when omitted. See [Separate Health Port](/configuration/health-checks#separate-health-port) |
| **timeout**  | number | `interval - 1` | The number of seconds to wait for a successful response. Defaults to `interval` minus one |
| **disable**  | bool | false       | Set to `true` to disable the health check entirely |

> Specifying **health** as a string will set the **path** and leave the other values as defaults.



### liveness

| Attribute  | Type   | Default | Description                                                                                      |
| ---------- | ------ | ------- | ------------------------------------------------------------------------------------------------ |
| **grace**    | number | 10       | The number of seconds to wait for a [Process](/reference/primitives/app/process) to start before starting liveness checks |
| **interval** | number | 5       | The number of seconds between health checks                                                      |
| **path**     | string |        | The path to request for health checks                                                            |
| **port**     | number or map | Main service port | The port the liveness probe connects to. Same form as `health.port`. Unlike readiness, liveness does **not** auto-inherit the main service scheme — set `scheme` explicitly when the probe needs HTTPS. See [Separate Health Port](/configuration/health-checks#separate-health-port) |
| **timeout**  | number | 5      | The number of seconds to wait for a successful response                                          |
| **successThreshold**  | number | 1      | The number of consecutive successful checks required to consider the probe successful             |
| **failureThreshold**  | number | 3      | The number of consecutive failed checks required before restarting the container                  |

> If you want to enable liveness check, you have to specify **path** and others are optional

### scale

| Attribute | Type   | Default | Description                                                                                                   |
| --------- | ------ | ------- | ------------------------------------------------------------------------------------------------------------- |
| **count**   | number | 1       | The number of [Processes](/reference/primitives/app/process) to run for this Service. For autoscaling use a range, e.g. **1-5**        |
| **cpu**     | number | 250     | The number of CPU units to reserve for [Processes](/reference/primitives/app/process) of this Service where 1000 units is a full CPU |
| **gpu**     | map    |         | The number/type of GPUs to reserve for [Processes](/reference/primitives/app/process) of this Service  |
| **memory**  | number | 512     | The number of MB of RAM to reserve for [Processes](/reference/primitives/app/process) of this Service                                |
| **targets** | map    |         | Target metrics to trigger autoscaling |
| **keda** | map    |         | KEDA event-driven autoscaling configuration. See [KEDA](/configuration/scaling/keda) |
| **vpa** | map    |         | Vertical Pod Autoscaler configuration. See [VPA](/configuration/scaling/vpa) |
| **limit** | map    |         | The maximum cpu or memory usage limit |

> Specifying **scale** as a number will set the **count** and leave the other values as defaults.

### scale.gpu

| Attribute | Type   | Default | Description                                                                                |
| --------- | ------ | ------- | ------------------------------------------------------------------------------------------ |
| **count**  | number |        | The number of GPUs to reserve for [Processes](/reference/primitives/app/process) of this Service    |
| **vendor** | string | nvidia | The GPU vendor to target for [Processes](/reference/primitives/app/process) of this Service |

> Specifying **gpu** as a number will set the **count** and leave the vendor as default.
> Specifying a **gpu** value and not specifying the cpu or memory to reserve will remove their defaults to purely reserve based on GPU.
> You should ensure that your Rack is running on GPU enabled instances (of the correct vendor) before specifying the **gpu** section in your convox.yml

### scale.targets

| Attribute | Type   | Default | Description                                                                                |
| --------- | ------ | ------- | ------------------------------------------------------------------------------------------ |
| **cpu**     | number |         | The percentage of CPU utilization to target for [Processes](/reference/primitives/app/process) of this Service    |
| **memory**  | number |         | The percentage of memory utilization to target for [Processes](/reference/primitives/app/process) of this Service |
| **external**  | map |         | The array of the external metrics based on which it will scale the Service |

### scale.limit

| Attribute  | Type   | Default       | Description                                                                                           |
| ---------- | ------ | ------------- | ----------------------------------------------------------------------------------------------------- |
| **cpu**    | number |               | The number of CPU units to limit for [Processes](/reference/primitives/app/process) of this Service where 1000 units is a full CPU |
| **memory** | number | <scale.memory> | The number of MB of RAM to limit for [Processes](/reference/primitives/app/process) of this Service     |

### scale.targets.[]external

| Attribute | Type   | Default | Description                                                                                |
| --------- | ------ | ------- | ------------------------------------------------------------------------------------------ |
| **name**     | string |         | The name of the metric |
| **matchLabels**  | map |         | Key-value labels for the metrics |
| **averageValue**  | number |         | The target value of the average of the metric across all relevant pods |
| **value**  | number |         | The target value of the metric |

```yaml
services:
  web:
    build: .
    port: 3000
    scale:
      count: 1-3
      targets:
        external:
          - name: "datadogmetric@default:web-requests"
            averageValue: 200
```



### securityContext

Container-level security settings. These settings control Linux security features for the container. Any field left unset is omitted from the rendered pod spec, leaving the Kubernetes default in effect.

| Attribute                    | Type    | Default | Description                                                                                                 |
| ---------------------------- | ------- | ------- | ----------------------------------------------------------------------------------------------------------- |
| **runAsNonRoot**             | boolean |         | Require the container to run as a non-root user                                                             |
| **runAsUser**                | number  |         | The UID to run the container as                                                                             |
| **runAsGroup**               | number  |         | The GID to run the container as                                                                             |
| **readOnlyRootFilesystem**   | boolean |         | Mount the root filesystem as read-only                                                                      |
| **allowPrivilegeEscalation** | boolean |         | Whether a process can gain more privileges than its parent process. Set to **false** for security hardening |
| **capabilities**             | map     |         | Linux capabilities to add or drop (see below)                                                               |
| **seccompProfile**           | string  |         | The seccomp profile to apply. Allowed values: `RuntimeDefault`, `Unconfined`                                |

```yaml
services:
  web:
    build: .
    port: 3000
    securityContext:
      runAsNonRoot: true
      runAsUser: 1000
      runAsGroup: 1000
      readOnlyRootFilesystem: true
      allowPrivilegeEscalation: false
      capabilities:
        drop:
          - ALL
      seccompProfile: RuntimeDefault
```

Notes:

- These settings apply to the service's Deployment pods, its Timer CronJob pods, and one-off containers created by `convox run <service>`. Build containers retain Convox's existing build-time security context (unconfined seccomp, optionally privileged) regardless of manifest configuration.
- The legacy top-level `privileged: true` flag still renders as `privileged: true` in the pod's security context. When `privileged: true` is set, Kubernetes grants the container all Linux capabilities at runtime; `capabilities.drop` has no effect in that mode. Do not mix `privileged: true` with `securityContext` hardening unless you understand that precedence.
- `seccompProfile: Localhost` is not currently supported because Convox does not expose the required `localhostProfile` path field.
- Namespaces with restrictive Kubernetes [Pod Security Admission](https://kubernetes.io/docs/concepts/security/pod-security-admission/) labels may reject pods that use `privileged: true` or that omit `runAsNonRoot` / `seccompProfile`. Configure `securityContext` to match the namespace's enforced profile.
- `seccompProfile: RuntimeDefault` requires the kubelet to provide a default seccomp profile. All managed providers Convox supports (EKS, GKE, AKS, DOKS) ship this profile; self-managed bare-metal clusters must provision it themselves.
- `readOnlyRootFilesystem: true` prevents the container from writing anywhere outside explicitly mounted volumes. Processes that cache to `$HOME` (AWS CLI credential cache, CUDA kernel cache at `~/.nv/ComputeCache`, many language runtimes) need a writable mount. Use `volumeOptions.emptyDir` with a `mountPath` targeting the cache directory.

### securityContext.capabilities

| Attribute | Type | Default | Description                                                                                       |
| --------- | ---- | ------- | ------------------------------------------------------------------------------------------------- |
| **drop**  | list |         | List of Linux capabilities to drop. Use `ALL` to drop all capabilities                            |
| **add**   | list |         | List of Linux capabilities to add back (typically after dropping `ALL`)                           |

Capability names are case-sensitive and must omit the `CAP_` prefix (use `NET_BIND_SERVICE`, not `CAP_NET_BIND_SERVICE`).

```yaml
services:
  web:
    build: .
    port: 3000
    securityContext:
      capabilities:
        drop:
          - ALL
        add:
          - NET_BIND_SERVICE
```

&nbsp;

### termination

| Attribute  | Type    | Default | Description                                                                                      |
| ---------- | ------- | ------- | ------------------------------------------------------------------------------------------------ |
| **grace**    | number  | 30      | The number of seconds to wait for [Processes](/reference/primitives/app/process) to gracefully exit before killing them |



### tls

| Attribute  | Type    | Default | Description                                                                          |
| ---------- | ------- | ------- | ------------------------------------------------------------------------------------ |
| **redirect** | boolean | true    | Whether or not HTTP requests should be redirected to HTTPS using a 308 response code |



### []volumeOptions

| Attribute  | Type    | Default | Description                                                                          |
| ---------- | ------- | ------- | ------------------------------------------------------------------------------------ |
| **emptyDir** | map |     | Configuration for [emptyDir](https://kubernetes.io/docs/concepts/storage/volumes/#emptydir) volume |
| **awsEfs** | map |     | Configuration for AWS Efs volume. To use this you have to enable efs csi driver in the rack |



### []volumeOptions.emptyDir

| Attribute  | Type    | Default | Description                                                                          |
| ---------- | ------- | ------- | ------------------------------------------------------------------------------------ |
| **id** | string |     | Required. Id of the volume. |
| **mountPath** | string |     | Required. Path in the service file system to mount the volume |
| **medium** | string |     | Optional. Specifies the emptyDir medium. Allowed values: `"Memory"` or `""` |
| **sizeLimit** | string |     | Optional. Kubernetes [resource quantity](https://kubernetes.io/docs/reference/kubernetes-api/common-definitions/quantity/) (e.g. `"2Gi"`). Kubernetes evicts the pod if usage of this emptyDir volume exceeds the limit. |

```yaml
environment:
  - PORT=3000
services:
  web:
    build: .
    port: 3000
    volumeOptions:
      - emptyDir:
          id: "test-vol"
          mountPath: "/my/test/vol"
          sizeLimit: "2Gi"
```



### []volumeOptions.awsEfs

| Attribute  | Type    | Default | Description                                                                          |
| ---------- | ------- | ------- | ------------------------------------------------------------------------------------ |
| **id** | string |     | Required. Id of the volume. |
| **mountPath** | string |     | Required. Path in the service file system to mount the volume |
| **accessMode** | string |     | Required. Specifies the access mode for the volume. Allowed values are: `ReadWriteOnce`, `ReadOnlyMany`, `ReadWriteMany` |
| **storageClass** | string |  | Storage class for the EFS volume |
| **volumeHandle** | string |  | Use an existing EFS access point instead of provisioning a new one. See [Volumes](/configuration/volumes) |

```yaml
environment:
  - PORT=3000
services:
  web:
    build: .
    port: 3000
    volumeOptions:
      - awsEfs:
          id: "efs-1"
          accessMode: ReadWriteMany
          mountPath: "/my/data/"
```



## Command Line Interface

### Listing Services
```bash
    $ convox services -a myapp
    SERVICE  DOMAIN                                PORTS
    web      web.convox.0a1b2c3d4e5f.convox.cloud  443:5000
```
### Scaling a Service
```bash
    $ convox scale web --count 3 --cpu 250 --memory 1024 -a myapp
    Scaling web... OK
```
### Restarting a Service
```bash
    $ convox services restart web -a myapp
    Restarting web... OK
```
> Restarting a Service will begin a rolling restart with graceful termination of each [Process](/reference/primitives/app/process) of the Service.

## See Also

- [Agents](/configuration/agents) for running a single process on every node (DaemonSet-style workloads)
- [Health Checks](/configuration/health-checks) for configuring readiness and liveness probes
- [Scaling](/configuration/scaling) for autoscaling, VPA, and workload placement
