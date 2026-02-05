---
title: "Convox to Kubernetes Resource Mapping"
draft: false
slug: convox-k8s-mapping
url: /reference/convox-k8s-mapping
---
# Convox to Kubernetes Resource Mapping Reference

## Overview

This document provides a comprehensive mapping of Convox concepts to their underlying Kubernetes implementations. It is intended for DevOps engineers who are familiar with Kubernetes and need to inspect, troubleshoot, or interact with their Convox-managed clusters using `kubectl`.

Convox abstracts Kubernetes complexity by mapping high-level concepts from your `convox.yml` to standard Kubernetes resources. Key principles of the Convox model on Kubernetes include:

-   **Namespace Isolation**: Each application is deployed into its own dedicated Kubernetes namespace, providing strong isolation. Rack-level system components also reside in their own namespace.
-   **Label-Driven Architecture**: Convox uses a consistent labeling scheme across all generated resources, making it easy to find and manage related components. The primary labels are `system=convox`, `rack=<rack-name>`, `app=<app-name>`, and `service=<service-name>`.
-   **Declarative Configuration**: Your `convox.yml` file is the single source of truth, which Convox translates into Kubernetes Deployments, Services, Ingresses, and other resource types.

Understanding these mappings allows you to leverage the full power of Kubernetes for advanced debugging and observability while still benefiting from the simplicity of the Convox developer experience.

## Namespace Mapping

Convox isolates resources by creating dedicated namespaces for the Rack system and for each application.

-   **Rack System Namespace**: Contains core Rack components like the API, controller, and router.
    -   **Pattern**: `<rack-name>-system`
    -   **Example**: `my-rack-system`

-   **Application Namespace**: Contains all Kubernetes resources specific to a single application, including its services, resources, timers, and environment variables.
    -   **Pattern**: `<rack-name>-<app-name>`
    -   **Example**: `my-rack-my-app`

**`kubectl` Commands:**

```bash
# List all namespaces managed by Convox for a given rack
kubectl get ns -l system=convox,rack=my-rack

# Set your kubectl context to a specific app's namespace for easier commands
kubectl config set-context --current --namespace=my-rack-my-app
```

## Quick Reference Table

| Convox Concept | K8s Resource(s) | Namespace | Naming Pattern | `kubectl` Command |
| :--- | :--- | :--- | :--- | :--- |
| **Rack** | `Namespace` | N/A | `<rack-name>-system` | `kubectl get ns -l type=rack` |
| **App** | `Namespace` | N/A | `<rack-name>-<app-name>` | `kubectl get ns -l type=app` |
| **Web Service** | `Deployment`, `Service`, `Ingress`, `HPA` | App | `<service-name>` | `kubectl get deploy,svc,ing,hpa -l service=<web>` |
| **Worker Service** | `Deployment`, `Service` (if ports) | App | `<service-name>` | `kubectl get deploy -l service=<worker>` |
| **Agent Service** | `DaemonSet`, `Service` (if ports) | App | `<service-name>` | `kubectl get ds -l service=<agent>` |
| **Service Pod** | `Pod` | App | `<service-name>-<rs-hash>-<pod-hash>` | `kubectl get pods -l service=<web>` |
| **Database Resource** | `Deployment`, `Service`, `PVC`, `ConfigMap` | App | `resource-<resource-name>` | `kubectl get all -l resource=<postgres>` |
| **Cache Resource** | `Deployment`, `Service`, `ConfigMap` | App | `resource-<resource-name>` | `kubectl get all -l resource=<redis>` |
| **Timer** | `CronJob` | App | `timer-<timer-name>` | `kubectl get cronjob -l system=convox` |
| **Environment** | `Secret` | App | `env-<service-name>` | `kubectl get secret env-<service-name>` |
| **Release** | `Label` on resources, `Image Tag` | App | `release=<release-id>` | `kubectl get pods -l release=<RABC1234DEF>` |
| **Custom Balancer** | `Service` (type: `LoadBalancer`) | App | `balancer-<balancer-name>` | `kubectl get svc -l type=balancer` |
| **EFS Volume** | `PersistentVolumeClaim` | App | `efs-<service-name>-<volume-id>` | `kubectl get pvc -l system=convox` |
| **emptyDir Volume** | `Pod.spec.volumes` | App | `ed-<volume-id>` | `kubectl describe pod <pod-name>` |
| **Init Container** | `Pod.spec.initContainers` | App | `init` | `kubectl describe pod <pod-name>` |
| **Pod Identity** | `ServiceAccount` | App | `<service-name>` | `kubectl get sa <service-name> -o yaml` |

## Complete convox.yml Example

The following `convox.yml` demonstrates every Convox concept covered in this document. Inline comments reference the Kubernetes resources each section generates. Use this as a cross-reference when inspecting your cluster with `kubectl`.

```yaml
# ============================================================================
# ENVIRONMENT
# ============================================================================
# Shared environment variables available to all services.
# Each service gets its own K8s Secret named "env-<service-name>" containing
# these values plus any service-level overrides.
#
# K8s Resource: Secret (env-web, env-worker, env-agent, env-jobs)
# kubectl: kubectl get secret env-web -n <rack>-<app> -o yaml

environment:
  - PORT=3000
  - RAILS_ENV=production
  - DATABASE_URL

# ============================================================================
# RESOURCES
# ============================================================================
# Each resource creates a set of K8s objects in the app namespace.
#
# Containerized resources → Deployment + Service (NodePort) + PVC + ConfigMap
#   Naming: resource-<name>  (e.g. resource-database, resource-cache)
#   kubectl: kubectl get deploy,svc,pvc,configmap -l resource=<name>
#
# Managed resources (rds-*, elasticache-*) are provisioned in the cloud
# provider and exposed to the app via the same ConfigMap / env var pattern.

resources:
  # --- Containerized Postgres -------------------------------------------
  # K8s: Deployment, Service (NodePort), PVC, ConfigMap
  # Injects DATABASE_URL, DATABASE_USER, DATABASE_PASS, etc.
  database:
    type: postgres
    options:
      version: 13
      storage: 20

  # --- Containerized Redis ----------------------------------------------
  # K8s: Deployment, Service (NodePort), ConfigMap  (no PVC — stateless)
  cache:
    type: redis

  # --- AWS RDS Postgres (managed) ---------------------------------------
  # K8s: ConfigMap only (cloud provider manages the instance)
  analytics-db:
    type: rds-postgres
    options:
      class: db.t3.large
      storage: 100
      version: 13
      encrypted: true
      durable: true
      deletionProtection: true
      backupRetentionPeriod: 7
      preferredBackupWindow: "02:00-03:00"

  # --- AWS ElastiCache Redis (managed) ----------------------------------
  # K8s: ConfigMap only
  sessions:
    type: elasticache-redis
    options:
      class: cache.t3.micro
      version: "6.2"
      encrypted: true
      transitEncryption: true
      durable: true

# ============================================================================
# SERVICES
# ============================================================================

services:
  # --- Web Service (public-facing) --------------------------------------
  # K8s Resources:
  #   Deployment    "web"          — runs the application containers
  #   Service       "web"          — ClusterIP routing to pods
  #   Ingress       "web"          — external HTTPS termination & routing
  #   HPA           "web"          — autoscaling (because scale.count is a range)
  #   Secret        "env-web"      — environment variables
  #   ServiceAccount "web"         — pod identity (if accessControl is set)
  #
  # kubectl: kubectl get deploy,svc,ing,hpa -l service=web

  web:
    build:
      path: .
      manifest: Dockerfile
    command: bin/web
    domain: ${WEB_HOST}
    port: 3000
    internal: false

    # Pod-level annotations → metadata.annotations on Pod and ServiceAccount
    annotations:
      - prometheus.io/scrape=true

    # Ingress-level annotations → metadata.annotations on Ingress
    ingressAnnotations:
      - nginx.ingress.kubernetes.io/limit-rpm=100

    # Custom K8s labels on all generated resources
    labels:
      convox.com/team: platform

    # Environment → merged into Secret "env-web"
    environment:
      - PORT=3000
      - WEB_HOST

    # Readiness probe → readinessProbe on the container
    health:
      path: /health
      grace: 10
      interval: 5
      timeout: 3

    # Liveness probe → livenessProbe on the container
    liveness:
      path: /liveness
      grace: 15
      interval: 10
      timeout: 5
      successThreshold: 1
      failureThreshold: 3

    # Certificate duration for auto-issued TLS cert
    certificate:
      duration: 2160h

    # TLS redirect → annotation on Ingress
    tls:
      redirect: true

    # Rolling deployment strategy → Deployment.spec.strategy
    deployment:
      minimum: 50
      maximum: 200

    # Termination grace → Pod.spec.terminationGracePeriodSeconds
    termination:
      grace: 45

    # Lifecycle hooks → Pod.spec.containers[].lifecycle
    lifecycle:
      preStop: "sleep 10"

    # DNS config → Pod.spec.dnsConfig
    dnsConfig:
      ndots: 5

    # Sticky sessions → Ingress session-cookie annotation
    sticky: true

    # Request timeout → Ingress proxy-read-timeout annotation
    timeout: 180

    # Whitelist → Ingress whitelist-source-range annotation
    whitelist: "10.0.0.0/16,192.168.1.0/24"

    # Scaling → Deployment replicas + HPA + resource requests/limits
    #   count range → HPA  (minReplicas / maxReplicas)
    #   cpu/memory  → container resources.requests
    #   limit       → container resources.limits
    #   targets     → HPA metrics
    scale:
      count: 2-10
      cpu: 256
      memory: 512
      limit:
        cpu: 512
        memory: 1024
      targets:
        cpu: 70
        memory: 80

    # Resource links → env vars injected from resource ConfigMaps
    resources:
      - database
      - cache
      - analytics-db
      - sessions

    # AWS Pod Identity → ServiceAccount with IAM role annotation
    #   kubectl: kubectl describe sa web
    accessControl:
      awsPodIdentity:
        policyArns:
          - "arn:aws:iam::aws:policy/AmazonS3ReadOnlyAccess"

    # EFS volume → PersistentVolumeClaim + volumeMount
    #   PVC name: efs-web-shared-uploads
    #   kubectl: kubectl get pvc -l system=convox
    volumeOptions:
      - awsEfs:
          id: "shared-uploads"
          accessMode: ReadWriteMany
          mountPath: "/app/public/uploads"

    # Init container → Pod.spec.initContainers
    initContainer:
      command: "bin/migrate"

  # --- Worker Service (background) -------------------------------------
  # K8s Resources:
  #   Deployment    "worker"       — runs background job processors
  #   Secret        "env-worker"   — environment variables
  #   (No Service/Ingress — no public ports)
  #
  # kubectl: kubectl get deploy -l service=worker

  worker:
    build: ./worker
    command: bin/worker
    environment:
      - QUEUE_NAME=default
    scale:
      count: 2
      cpu: 512
      memory: 1024
    resources:
      - database
      - cache

    # emptyDir volume → Pod.spec.volumes (ephemeral scratch space)
    volumeOptions:
      - emptyDir:
          id: "tmp-data"
          mountPath: "/tmp/processing"

  # --- Agent Service (DaemonSet) ----------------------------------------
  # K8s Resources:
  #   DaemonSet     "agent"        — one pod per node
  #   Service       "agent"        — because ports are defined
  #   Secret        "env-agent"    — environment variables
  #
  # kubectl: kubectl get ds -l service=agent

  agent:
    build: ./agent
    agent: true
    ports:
      - 8125/udp
      - 8126

  # --- Jobs Template Service (scaled to zero) ---------------------------
  # K8s Resources:
  #   Deployment    "jobs"         — 0 replicas (template only)
  #   Secret        "env-jobs"     — environment variables
  #
  # No running pods — used only as the image source for Timers.

  jobs:
    build: ./jobs
    scale:
      count: 0
    resources:
      - database

# ============================================================================
# TIMERS
# ============================================================================
# Each timer creates a K8s CronJob in the app namespace.
#
# Naming: timer-<timer-name>
# kubectl: kubectl get cronjobs -l system=convox
#          kubectl get jobs      -l name=<timer-name>

timers:
  # Simple scheduled task → CronJob "timer-cleanup"
  cleanup:
    command: bin/cleanup
    schedule: "0 3 * * *"
    service: jobs
    concurrency: forbid

  # Parallel timer → CronJob "timer-data-import" with parallelism=4
  # Each replica gets a TIMER_INDEX env var (0, 1, 2, 3)
  data-import:
    command: bin/import
    schedule: "0 * * * *"
    service: jobs
    parallelCount: 4
    concurrency: forbid
    annotations:
      - monitoring.example.com/alert=true

# ============================================================================
# BALANCERS
# ============================================================================
# Each balancer creates a K8s Service of type LoadBalancer.
#
# Naming: balancer-<balancer-name>
# kubectl: kubectl get svc -l type=balancer

balancers:
  # TCP balancer → Service (LoadBalancer) "balancer-tcp-lb"
  tcp-lb:
    annotations:
      - service.beta.kubernetes.io/aws-load-balancer-type=nlb
    service: worker
    ports:
      5000: 3001
      5002: 3002
```

### How convox.yml Maps to Kubernetes

The diagram below summarizes the relationship between the `convox.yml` sections above and the Kubernetes resources Convox generates:

```
convox.yml Section        Kubernetes Resources Created
─────────────────         ──────────────────────────────────────────────
environment:         ──►  Secret         (env-<service>)
resources:           ──►  Deployment + Service + PVC + ConfigMap
                          (or cloud-managed + ConfigMap only)
services:
  web: (port set)    ──►  Deployment + Service + Ingress + HPA
  worker: (no port)  ──►  Deployment
  agent: (agent)     ──►  DaemonSet + Service
  jobs: (count: 0)   ──►  Deployment (0 replicas, template only)
  ├─ scale           ──►  HPA + resources.requests/limits
  ├─ health          ──►  readinessProbe
  ├─ liveness        ──►  livenessProbe
  ├─ volumeOptions   ──►  PVC (EFS) or emptyDir volume
  ├─ initContainer   ──►  Pod.spec.initContainers
  ├─ accessControl   ──►  ServiceAccount + IAM annotation
  └─ termination     ──►  terminationGracePeriodSeconds
timers:              ──►  CronJob        (timer-<name>)
balancers:           ──►  Service type=LoadBalancer (balancer-<name>)
```

## Detailed Mappings

### Rack

A Convox Rack's system components are deployed into a dedicated Kubernetes namespace.

-   **K8s Resource**: `Namespace`
-   **Naming Pattern**: `<rack-name>-system`
-   **`kubectl` Commands**:
    ```bash
    # View the rack system namespace
    kubectl get namespace my-rack-system

    # View all system components running in the rack
    kubectl get all -n my-rack-system
    ```

### Apps

Each Convox App is deployed into its own dedicated Kubernetes namespace.

-   **K8s Resource**: `Namespace`
-   **Naming Pattern**: `<rack-name>-<app-name>`
-   **`kubectl` Commands**:
    ```bash
    # View the namespace for a specific app
    kubectl get namespace my-rack-my-app

    # View all resources for a specific app
    kubectl get all -n my-rack-my-app
    ```

### Services

Services defined in `convox.yml` are the core of your application. They map to different Kubernetes workload resources based on their configuration.

#### Web Services

A standard, publicly-exposed service with an HTTP/HTTPS endpoint.

-   **K8s Resources**: `Deployment`, `Service` (type `ClusterIP`), `Ingress`, `HorizontalPodAutoscaler` (if autoscaling is enabled).
-   **Naming Pattern**: All resources are named after the service (e.g., `web`).
-   **YAML Snippet (`Deployment`)**:
    ```yaml
    apiVersion: apps/v1
    kind: Deployment
    metadata:
      name: web # from convox.yml: services.web
      namespace: my-rack-my-app
      labels:
        app: my-app
        rack: my-rack
        service: web
        system: convox
    spec:
      replicas: 2 # from convox.yml: scale.count
      selector:
        matchLabels:
          service: web
      template:
        metadata:
          labels:
            service: web
        spec:
          terminationGracePeriodSeconds: 45 # from convox.yml: termination.grace
          containers:
            - name: my-app
              image: 123456789012.dkr.ecr.us-east-1.amazonaws.com/my-rack/my-app:web.RABC1234DEF
              ports:
                - containerPort: 3000 # from convox.yml: services.web.port
              readinessProbe: # from convox.yml: services.web.health
                httpGet:
                  path: /health
                  port: 3000
                initialDelaySeconds: 10
              livenessProbe: # from convox.yml: services.web.liveness
                httpGet:
                  path: /liveness
                  port: 3000
                initialDelaySeconds: 15
    ```
-   **`kubectl` Commands**:
    ```bash
    # Get all primary resources for the 'web' service
    kubectl get deploy,svc,ing,hpa -n my-rack-my-app -l service=web

    # Describe the deployment to see its status and configuration
    kubectl describe deployment web -n my-rack-my-app

    # View the running pods for the service
    kubectl get pods -n my-rack-my-app -l service=web
    ```

#### Worker Services

A background service that does not expose a public endpoint.

-   **K8s Resources**: `Deployment`. A `Service` (type `ClusterIP`) is created only if `ports` are defined for internal communication or for a custom balancer.
-   **Naming Pattern**: `<service-name>` (e.g., `worker`).
-   **YAML Snippet (`Deployment`)**:
    ```yaml
    apiVersion: apps/v1
    kind: Deployment
    metadata:
      name: worker # from convox.yml: services.worker
      namespace: my-rack-my-app
    spec:
      replicas: 2 # from convox.yml: scale.count
      template:
        spec:
          containers:
            - name: my-app
              command: ["node", "worker/index.js"] # from convox.yml: services.worker.command
              # No 'ports' section unless defined in convox.yml
    ```
-   **`kubectl` Commands**:
    ```bash
    # Get the deployment for the 'worker' service
    kubectl get deployment worker -n my-rack-my-app

    # Tail logs from all worker pods
    kubectl logs -f -n my-rack-my-app -l service=worker
    ```

#### Agent Services

A service that runs one pod on every node in the cluster.

-   **K8s Resource**: `DaemonSet`.
-   **Naming Pattern**: `<service-name>` (e.g., `agent`).
-   **YAML Snippet (`DaemonSet`)**:
    ```yaml
    apiVersion: apps/v1
    kind: DaemonSet
    metadata:
      name: agent # from convox.yml: services.agent
      namespace: my-rack-my-app
    spec:
      template:
        spec:
          containers:
            - name: my-app
              ports:
                - containerPort: 8125
                  hostPort: 8125 # from convox.yml: services.agent.ports
                  protocol: UDP
                - containerPort: 8126
                  hostPort: 8126
                  protocol: TCP
    ```
-   **`kubectl` Commands**:
    ```bash
    # Get the DaemonSet for the 'agent' service
    kubectl get daemonset agent -n my-rack-my-app

    # Check which nodes the agent pods are running on
    kubectl get pods -n my-rack-my-app -l service=agent -o wide
    ```

### Resources

Convox can provision containerized or cloud-managed (e.g., AWS RDS) backing services.

#### Stateful Resources (e.g., Postgres, MySQL)

-   **K8s Resources**: `Deployment`, `Service` (type `NodePort`), `PersistentVolumeClaim` (PVC), and a `ConfigMap` for connection details.
-   **Naming Pattern**: `resource-<resource-name>` (e.g., `resource-main-pg`).
-   **YAML Snippet (`Deployment`)**:
    ```yaml
    apiVersion: apps/v1
    kind: Deployment
    metadata:
      name: resource-main-pg # from convox.yml: resources.main-pg
      labels:
        type: resource
        resource: main-pg
    spec:
      strategy:
        type: Recreate # Ensures only one pod is running at a time
      template:
        spec:
          containers:
            - name: postgres
              image: postgres:13 # from convox.yml: resources.main-pg.type/version
              volumeMounts:
                - name: data
                  mountPath: /var/lib/postgresql/data
          volumes:
            - name: data
              persistentVolumeClaim:
                claimName: resource-main-pg
    ```
-   **`kubectl` Commands**:
    ```bash
    # Get all components for a postgres resource
    kubectl get deploy,svc,pvc,configmap -n my-rack-my-app -l resource=main-pg

    # View the connection details stored in the ConfigMap
    kubectl get configmap resource-main-pg -n my-rack-my-app -o yaml
    ```

#### Stateless Resources (e.g., Redis, Memcached)

-   **K8s Resources**: `Deployment`, `Service` (type `NodePort`), and a `ConfigMap`. No `PersistentVolumeClaim` is created.
-   **`kubectl` Commands**:
    ```bash
    # Get all components for a redis resource
    kubectl get deploy,svc,configmap -n my-rack-my-app -l resource=redis
    ```

### Timers

Scheduled tasks defined in `convox.yml`.

-   **K8s Resource**: `CronJob`
-   **Naming Pattern**: `timer-<timer-name>` (e.g., `timer-hourly-cleanup`).
-   **YAML Snippet (`CronJob`)**:
    ```yaml
    apiVersion: batch/v1
    kind: CronJob
    metadata:
      name: timer-hourly-cleanup # from convox.yml: timers.hourly-cleanup
      namespace: my-rack-my-app
    spec:
      schedule: "0 * * * *" # from convox.yml: schedule
      jobTemplate:
        spec:
          template:
            spec:
              containers:
                - name: my-app
                  image: 123456789012.dkr.ecr.us-east-1.amazonaws.com/my-rack/my-app:worker.RABC1234DEF # from convox.yml: service
                  args: ["./scripts/hourly-cleanup.sh"] # from convox.yml: command
              restartPolicy: Never
    ```
-   **`kubectl` Commands**:
    ```bash
    # List all CronJobs for the app
    kubectl get cronjobs -n my-rack-my-app

    # View the jobs created by a CronJob
    kubectl get jobs -n my-rack-my-app --watch

    # View logs from the most recent job pod for a timer
    kubectl logs -n my-rack-my-app -l name=hourly-cleanup
    ```

### Environment

Environment variables are stored as Kubernetes Secrets and injected into pods.

-   **K8s Resource**: `Secret`
-   **Naming Pattern**: `env-<service-name>`
-   **Injection Method**: The `envFrom` field in the container spec references the Secret.
-   **YAML Snippet (`Pod` spec)**:
    ```yaml
    spec:
      containers:
        - name: my-app
          envFrom:
            - secretRef:
                name: env-web # Contains all env vars for the 'web' service
    ```
-   **`kubectl` Commands**:
    ```bash
    # View a service's environment secret (values are base64 encoded)
    kubectl get secret env-web -n my-rack-my-app -o yaml

    # Decode a specific secret value
    kubectl get secret env-web -n my-rack-my-app -o jsonpath='{.data.API_KEY}' | base64 --decode
    ```

### Volumes

Convox supports both ephemeral (`emptyDir`) and persistent (`awsEfs`) volumes.

-   **`emptyDir`**: Maps directly to an `emptyDir` volume in the Pod spec.
-   **`awsEfs`**: Creates a `PersistentVolumeClaim` (PVC) which is then mounted by the Pod.
-   **Naming Pattern (PVC)**: `efs-<service-name>-<volume-id>`
-   **YAML Snippet (`Pod` spec for EFS)**:
    ```yaml
    spec:
      containers:
      - name: my-app
        volumeMounts:
          - name: efs-shared-data # from convox.yml: volumeOptions.awsEfs.id
            mountPath: /data/shared # from convox.yml: mountPath
      volumes:
        - name: efs-shared-data
          persistentVolumeClaim:
            claimName: efs-storage-shared-data
    ```
-   **`kubectl` Commands**:
    ```bash
    # List all PersistentVolumeClaims in the app namespace
    kubectl get pvc -n my-rack-my-app

    # Describe a PVC to see its status and the PersistentVolume it's bound to
    kubectl describe pvc efs-storage-shared-data -n my-rack-my-app
    ```

### Scaling

Service scaling settings in `convox.yml` map to `Deployment` replicas and `HorizontalPodAutoscaler` resources.

-   **`scale.count` (fixed)**: Sets `spec.replicas` in the `Deployment`.
-   **`scale.count` (range)**: Creates a `HorizontalPodAutoscaler` (HPA).
-   **`scale.cpu`/`memory`**: Sets `spec.template.spec.containers[].resources.requests` and `.limits`.
-   **`scale.targets`**: Sets the target utilization in the `HPA` spec.
-   **YAML Snippet (`HPA`)**:
    ```yaml
    apiVersion: autoscaling/v2
    kind: HorizontalPodAutoscaler
    metadata:
      name: web # from convox.yml: services.web
    spec:
      scaleTargetRef:
        apiVersion: apps/v1
        kind: Deployment
        name: web
      minReplicas: 2 # from convox.yml: scale.count (min)
      maxReplicas: 5 # from convox.yml: scale.count (max)
      metrics:
        - type: Resource
          resource:
            name: cpu
            target:
              type: Utilization
              averageUtilization: 70 # from convox.yml: scale.targets.cpu
    ```
-   **`kubectl` Commands**:
    ```bash
    # Describe the HPA to see its current metrics and scaling events
    kubectl describe hpa web -n my-rack-my-app

    # Manually scale a deployment (will be overridden by HPA if active)
    kubectl scale deployment/web --replicas=3 -n my-rack-my-app
    ```

### Balancers

Custom balancers defined in `convox.yml`.

-   **K8s Resource**: `Service` of type `LoadBalancer`.
-   **Naming Pattern**: `balancer-<balancer-name>`
-   **YAML Snippet (`Service`)**:
    ```yaml
    apiVersion: v1
    kind: Service
    metadata:
      name: balancer-tcp-lb # from convox.yml: balancers.tcp-lb
      labels:
        type: balancer
    spec:
      type: LoadBalancer
      selector:
        service: worker # from convox.yml: balancers.tcp-lb.service
      ports:
        - name: "6000"
          port: 6000 # from convox.yml: ports
          targetPort: 6001
          protocol: TCP
    ```
-   **`kubectl` Commands**:
    ```bash
    # Get all custom balancers for an app
    kubectl get svc -n my-rack-my-app -l type=balancer

    # Get the external IP or hostname of a balancer
    kubectl get svc balancer-tcp-lb -n my-rack-my-app -o jsonpath='{.status.loadBalancer.ingress[0].hostname}'
    ```

### Pod Identity (AWS)

The `accessControl.awsPodIdentity` feature maps to an EKS Pod Identity association.

-   **K8s Resources**: `ServiceAccount` with AWS role annotations, and specific `volumeMounts` and `env` vars injected into the `Pod`.
-   **YAML Snippet (`ServiceAccount`)**:
    ```yaml
    apiVersion: v1
    kind: ServiceAccount
    metadata:
      name: s3-service # from convox.yml: services.s3-service
      namespace: my-rack-my-app
      annotations:
        # This annotation is added by Convox to associate the SA with an IAM role
        eks.amazonaws.com/role-arn: arn:aws:iam::123456789012:role/convox-my-rack-my-app-s3-service
    ```
-   **`kubectl` Commands**:
    ```bash
    # Describe the ServiceAccount to see its annotations
    kubectl describe sa s3-service -n my-rack-my-app

    # Exec into a pod to verify AWS identity
    kubectl exec -it <pod-name> -n my-rack-my-app -- aws sts get-caller-identity
    ```

## Troubleshooting Cheat Sheet

### Finding Things

Use labels to quickly find all resources related to a specific Convox primitive.

| Task | `kubectl` Command |
| :--- | :--- |
| Find all resources for an app | `kubectl get all -n my-rack-my-app` |
| Find all pods for a service | `kubectl get pods -n my-rack-my-app -l service=web` |
| Find all pods for a release | `kubectl get pods -n my-rack-my-app -l release=RABC1234DEF` |
| Find the deployment for a service | `kubectl get deployment -n my-rack-my-app -l service=web` |
| Find the CronJob for a timer | `kubectl get cronjob -n my-rack-my-app -l system=convox,type=timer` |
| Find the Service for a resource | `kubectl get svc -n my-rack-my-app -l resource=main-pg` |

### Common Issues

| Convox Symptom | K8s Root Cause | Investigation Steps (`kubectl`) |
| :--- | :--- | :--- |
| **Deployment fails or rolls back** | Pods are not becoming "Ready". Common reasons: `CrashLoopBackOff`, `ImagePullBackOff`, failing readiness probes. | 1. `kubectl get pods -n my-rack-my-app -l service=<service>` (Check STATUS) <br> 2. `kubectl describe pod <pod-name> -n my-rack-my-app` (Look at Events) <br> 3. `kubectl logs <pod-name> -n my-rack-my-app` (Check for application errors) |
| **Service is unreachable (503 error)** | No healthy pods are available to receive traffic. Service selector might be wrong, or all pods are failing health checks. | 1. `kubectl describe svc <service> -n my-rack-my-app` (Check Endpoints) <br> 2. `kubectl get pods -l service=<service> -n my-rack-my-app` (Ensure pods are Running/Ready) <br> 3. `kubectl describe ing <service> -n my-rack-my-app` (Check backend service/port) |
| **App has no logs** | Pods are in `CrashLoopBackOff` and restart before logging, or the logging driver has issues. | 1. `kubectl logs --previous <pod-name> -n my-rack-my-app` (Get logs from previous failed container) <br> 2. `kubectl describe pod <pod-name> -n my-rack-my-app` (Check `Restart Count` and `Events`) |
| **`convox run` fails** | Insufficient cluster resources (`Pending` state), image pull error, or command exits immediately. | 1. `kubectl get pods -n my-rack-my-app` (Find the one-off pod) <br> 2. `kubectl describe pod <pod-name> -n my-rack-my-app` (Check Events for scheduling errors) <br> 3. `kubectl logs <pod-name> -n my-rack-my-app` (See command output/errors) |
| **Timer job never runs** | CronJob schedule is invalid, or the job fails to create pods. | 1. `kubectl describe cronjob timer-<timer-name> -n my-rack-my-app` (Check schedule and events) <br> 2. `kubectl get jobs -n my-rack-my-app` (See if jobs are being created) <br> 3. `kubectl logs $(kubectl get pods -l name=<timer-name> -o jsonpath='{.items[0].metadata.name}')` |
| **Autoscaling not working** | HPA can't fetch metrics, or resource requests are not set on pods. | 1. `kubectl describe hpa <service> -n my-rack-my-app` (Look at Events and Metrics) <br> 2. `kubectl get --raw "/apis/metrics.k8s.io/v1beta1/namespaces/my-rack-my-app/pods" \| jq .` (Check if metrics are available) |
| **Persistent data is lost** | PVC is not configured correctly, or the pod is not mounting the volume. | 1. `kubectl get pvc -n my-rack-my-app` (Check PVC status is `Bound`) <br> 2. `kubectl describe pod <pod-name> -n my-rack-my-app` (Verify `Volumes` and `Volume Mounts` sections) |
