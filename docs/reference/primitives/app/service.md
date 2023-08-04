---
title: "Service"
draft: false
slug: Service
url: /reference/primitives/app/service
---
# Service

A Service is a horizontally-scalable collection of durable [Processes](/reference/primitives/app/process).

[Processes](/reference/primitives/app/process) that belong to a Service are automatically restarted upon termination.

Services can be scaled to a static count or autoscaled in a range based on metrics.

## Definition

```html
services:
  web:
    build: .
    health: /check
    port: 5000
    scale: 3
```

```html
services:
  grpc:
    build: .
    port: grpc:5551
    grpcHealthEnabled: true
```

```html
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
    drain: 10
    environment:
      - FOO
      - BAR=qux
    health:
      grace: 10
      interval: 5
      path: /check
      timeout: 3
    internal: false
    labels:
      convox.com/test: true
    lifecycle:
      preStop: "sleep 10"
      postStart: "sleep 10"
    port: 5000
    ports:
      - 5001
      - 5002
    privileged: false
    scale:
      count: 1-3
      cpu: 128
      memory: 512
      targets:
        cpu: 50
        memory: 80
        external:
          - name: "datadogmetric@default:web-requests"
            averageValue: 200
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
| **build**       | string/map | .                   | Build definition (see below)                                                                                                                                            |
| **certificate**| map         |                     | Define certificate parameters                                                                       |
| **command**     | string     | **CMD** of Dockerfile | The command to run to start a [Process](/reference/primitives/app/process) for this Service                                                                       |
| **deployment**  | map        |                     | Manual control over deployment parameters                                                                                                  |
| **domain**      | string     |                     | A custom domain(s) (comma separated) to route to this Service                                                                              |
| **drain**       | number     |                     | The number of seconds to wait for connections to drain when terminating a [Process](/reference/primitives/app/process) of this Service. Only applies for version 2 rack services. For version 3 rack services use termination grace period **termination.grace** |
| **environment** | list       |                     | A list of environment variables (with optional defaults) to populate from the [Release](/reference/primitives/app/release) environment                            |
| **grpcHealthEnabled** | boolean   |      false          | Enables liveliness health check for grpc. It should follow the [grpc health protocol](https://github.com/grpc/grpc/blob/master/doc/health-checking.md) (ref: [k8s](https://kubernetes.io/docs/tasks/configure-pod-container/configure-liveness-readiness-startup-probes/#define-a-grpc-liveness-probe))|
| **health**      | string/map | /                   | Health check definition (see below)                                                                                                        |
| **image**       | string     |                     | An external Docker image to use for this Service (supercedes **build**)                                                                      |
| **internal**    | boolean    | false               | Set to **true** to make this Service only accessible inside the Rack                                                                         |
| **internalRouter** | boolean    | false               | Set it to **true** to make this Service only accessible using internal loadbalancer. You also have to set the rack parameter [internal_router](/installation/production-rack/aws) to **true**                 |
| **labels** |  map  |       | Custom labels for k8s resources. See here for (syntax and character set)[https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/#syntax-and-character-set]. Also following keys are reserved: `system`, `rack`, `app`, `name`, `service`, `release`, `type` |
| **lifecycle** |  map  |       | The prestop and poststart hooks enable running commands before terminating and after starting the container, respectively |
| **port**        | string     |                     | The port that the default Rack balancer will use to [route incoming traffic](/configuration/load-balancers). For grpc service specify the scheme: `grpc:5051`|
| **ports**       | list       |                     | A list of ports available for internal [service discovery](/configuration/service-discovery) or custom [Balancers](/reference/primitives/app/balancer) |
| **privileged**  | boolean    | true                | Set to **false** to prevent [Processes](/reference/primitives/app/process) of this Service from running as root inside their container                              |
| **scale**       | map        | 1                   | Define scaling parameters (see below)                                                                                                      |
| **singleton**   | boolean    | false               | Set to **true** to prevent extra [Processes](/reference/primitives/app/process) of this Service from being started during deployments                               |
| **sticky**      | boolean    | false               | Set to **true** to enable sticky sessions                                                                                                    |
| **termination** | map        |                     | Termination related configuration                                                                                                          |
| **test**        | string     |                     | A command to run to test this Service when running **convox test**                                                                           |
| **timeout**     | number     | 60                  | Timeout period (in seconds) for reading/writing requests to/from your service                                                              |
| **tls**         | map        |                     | TLS-related configuration                                                                                                                  |
| **whitelist**   | string     |                     | Comma delimited list of CIDRs, e.g. `10.0.0.0/24,172.10.0.1`, to allow access to the service                                                                                                                  |

> Environment variables declared on `convox.yml` will be populated for a Service.

#### *annotations
You can use annotations to attach arbitrary non-identifying metadata to objects. Clients such as tools and libraries can retrieve this metadata. On Convox, annotations will reflect in pods and service accounts.

Here are some examples of information that can be recorded in annotations:
- Build, release, or image information like timestamps, release IDs, git branch, PR numbers, image hashes, and registry address.
- Fields managed by a declarative configuration layer. Attaching these fields as annotations distinguishes them from default values set by clients or servers, and from auto-generated fields and fields set by auto-sizing or auto-scaling systems.
- User or tool/system provenance information, such as URLs of related objects from other ecosystem components.
- Configure a service to assume an IAM Role(AWS and GCP only). For example:

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

### build

| Attribute  | Type   | Default    | Description                                                   |
| ---------- | ------ | ---------- | ------------------------------------------------------------- |
| **manifest** | string | Dockerfile | The filename of the Dockerfile                                |
| **path**     | string | .          | The path (relative to **convox.yml**) to build for this Service |

> Specifying **build** as a string will set the **path** and leave the other values as defaults.

### certificate

| Attribute  | Type   | Default    | Description                                                   |
| ---------- | ------ | ---------- | ------------------------------------------------------------- |
| **duration** | string | 2160h | Certificate renew frequency period                                |

### deployment

| Attribute | Type   | Default | Description                                                                      |
| --------- | ------ | ------- | -------------------------------------------------------------------------------- |
| **maximum** | number | 200     | The maximum percentage of Processes to allow during rolling deploys              |
| **minimum** | number | 50      | The minimum percentage of healthy Processes to keep alive during rolling deploys |

&nbsp;

### lifecycle

| Attribute | Type   | Default | Description                                                                                |
| --------- | ------ | ------- | ------------------------------------------------------------------------------------------ |
| **perStop**     | string |         | The pre stop command |
| **postStart**  | string |         | The post stop command |

&nbsp;

### health

| Attribute  | Type   | Default | Description                                                                                      |
| ---------- | ------ | ------- | ------------------------------------------------------------------------------------------------ |
| **grace**    | number | 5       | The number of seconds to wait for a [Process](/reference/primitives/app/process) to start before starting health checks |
| **interval** | number | 5       | The number of seconds between health checks                                                      |
| **path**     | string | /       | The path to request for health checks                                                            |
| **timeout**  | number | 4       | The number of seconds to wait for a successful response                                          |

> Specifying **health** as a string will set the **path** and leave the other values as defaults.

### scale

| Attribute | Type   | Default | Description                                                                                                   |
| --------- | ------ | ------- | ------------------------------------------------------------------------------------------------------------- |
| **count**   | number | 1       | The number of [Processes](/reference/primitives/app/process) to run for this Service. For autoscaling use a range, e.g. **1-5**        |
| **cpu**     | number | 256     | The number of CPU units to reserve for [Processes](/reference/primitives/app/process) of this Service where 1000 units is a full CPU |
| **gpu**     | map    |         | The number/type of GPUs to reserve for [Processes](/reference/primitives/app/process) of this Service  |
| **memory**  | number | 512     | The number of MB of RAM to reserve for [Processes](/reference/primitives/app/process) of this Service                                |
| **targets** | map    |         | Target metrics to trigger autoscaling                                                                         |

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

### scale.targets.[]external

| Attribute | Type   | Default | Description                                                                                |
| --------- | ------ | ------- | ------------------------------------------------------------------------------------------ |
| **name**     | string |         | The name of the metric |
| **matchLabels**  | map |         | Key value lablels for the metrics |
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

&nbsp;

### termination

| Attribute  | Type    | Default | Description                                                                                      |
| ---------- | ------- | ------- | ------------------------------------------------------------------------------------------------ |
| **grace**    | number  | 30      | The number of seconds to wait for [Processes](/reference/primitives/app/process) to gracefully exit before killing them |

&nbsp;

### tls

| Attribute  | Type    | Default | Description                                                                          |
| ---------- | ------- | ------- | ------------------------------------------------------------------------------------ |
| **redirect** | boolean | true    | Whether or not HTTP requests should be redirected to HTTPS using a 308 response code |

&nbsp;

## Command Line Interface

### Listing Services
```html
    $ convox services -a myapp
    SERVICE  DOMAIN                                PORTS
    web      web.convox.0a1b2c3d4e5f.convox.cloud  443:5000
```
### Scaling a Service
```html
    $ convox scale web --count 3 --cpu 256 --memory 1024 -a myapp`1
    Scaling web... OK
```
### Restarting a Service
```html
    $ convox services restart web -a myapp
    Restarting web... OK
```
> Restarting a Service will begin a rolling restart with graceful termination of each [Process](/reference/primitives/app/process) of the Service.
