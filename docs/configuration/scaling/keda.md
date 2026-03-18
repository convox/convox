---
title: "KEDA Autoscaling"
slug: keda
url: /configuration/scaling/keda
---

# KEDA Autoscaling

KEDA (Kubernetes Event-Driven Autoscaling) extends Convox autoscaling with event-driven triggers. Standard Convox autoscaling targets CPU and memory utilization, which works well for request-driven services. KEDA goes further by enabling two capabilities that standard autoscaling cannot provide:

- **Scale to zero**: Services can scale down to zero replicas when idle, eliminating compute costs for workloads that are not actively processing. When new events arrive (e.g. messages in a queue), KEDA spins up replicas automatically.
- **Scale from external signals**: Instead of reacting to pod-level CPU or memory, KEDA scales based on external event sources: queue depth, cron schedules, Prometheus queries, or any of 60+ supported scalers. This means your services scale in response to actual demand signals rather than lagging resource utilization.

Any scaler listed in the [KEDA Scalers documentation](https://keda.sh/docs/2.19/scalers/) can be configured via the `scale.keda.triggers` block in `convox.yml`.

## Prerequisites

Enable KEDA on your rack:

```bash
$ convox rack params set keda_enable=true -r rackName
Setting parameters... OK
```

## Configuration

Define KEDA triggers in the `scale.keda` section of your service in `convox.yml`:

```yaml
services:
  worker:
    build: .
    command: bin/worker
    scale:
      count: 1-10
      keda:
        pollingInterval: 30
        cooldownPeriod: 300
        triggers:
          - type: aws-sqs-queue
            metadata:
              queueURL: https://sqs.us-east-1.amazonaws.com/123456789/my-queue
              queueLength: "5"
              awsRegion: "us-east-1"
```

The `count` range defines the minimum and maximum replicas. KEDA scales within this range based on trigger activity.

### keda

| Attribute | Type | Default | Description |
| --------- | ---- | ------- | ----------- |
| **triggers** | list | | **Required.** List of KEDA trigger definitions (see below) |
| **pollingInterval** | number | 30 | How frequently KEDA checks the trigger source (seconds) |
| **cooldownPeriod** | number | 300 | Time to wait after the last trigger activation before scaling down (seconds) |
| **initialCooldownPeriod** | number | | Cooldown period before the first scale-down after deployment (seconds) |
| **idleReplicaCount** | number | | Number of replicas when no triggers are active. Set to `0` to scale to zero |
| **advanced** | map | | Advanced KEDA ScaledObject configuration |
| **fallback** | map | | Fallback behavior when metrics are unavailable |

### Trigger Definition

Each trigger requires a `type` and a `metadata` map. The available trigger types and their metadata fields are defined in the [KEDA Scalers documentation](https://keda.sh/docs/2.19/scalers/).

| Attribute | Type | Description |
| --------- | ---- | ----------- |
| **type** | string | The KEDA scaler type (e.g. `aws-sqs-queue`, `prometheus`, `cron`) |
| **metadata** | map | Scaler-specific configuration key-value pairs |
| **authenticationRef** | map | Optional reference to a TriggerAuthentication resource |

## Examples

### SQS Queue Worker

A common pattern for queue-driven architectures: scale a worker service based on the number of messages in an SQS queue. With `idleReplicaCount: 0`, the service runs zero replicas when the queue is empty, saving compute costs entirely during idle periods.

```yaml
services:
  worker:
    build: .
    command: bin/process-queue
    scale:
      count: 0-10
      keda:
        pollingInterval: 15
        cooldownPeriod: 120
        idleReplicaCount: 0
        triggers:
          - type: aws-sqs-queue
            metadata:
              queueURL: https://sqs.us-east-1.amazonaws.com/123456789/jobs
              queueLength: "5"
              awsRegion: "us-east-1"
```

KEDA checks the queue every 15 seconds. When messages appear, it scales up so that each replica handles roughly 5 messages. When the queue empties and stays empty for the 120-second cooldown, the service scales back to zero.

### Cron-Based Scaling

Scale services on a predictable schedule. This is useful for services that experience consistent traffic patterns, such as higher load during business hours:

```yaml
services:
  api:
    build: .
    port: 3000
    scale:
      count: 2-20
      keda:
        triggers:
          - type: cron
            metadata:
              timezone: America/New_York
              start: "0 8 * * 1-5"
              end: "0 18 * * 1-5"
              desiredReplicas: "10"
```

This scales the API to 10 replicas during weekday business hours (8 AM - 6 PM ET) and back to the minimum of 2 outside those windows.

### AWS CloudWatch Metrics

Scale based on any AWS CloudWatch metric. This is useful for scaling on ALB request counts, custom application metrics published to CloudWatch, or any other AWS-native signal. No additional infrastructure is required since all AWS racks have CloudWatch, and KEDA authenticates automatically via the rack's IAM role.

```yaml
services:
  api:
    build: .
    port: 3000
    scale:
      count: 2-15
      keda:
        pollingInterval: 60
        triggers:
          - type: aws-cloudwatch
            metadata:
              namespace: AWS/ApplicationELB
              dimensionName: LoadBalancer
              dimensionValue: app/my-alb/1234567890abcdef
              metricName: RequestCountPerTarget
              targetMetricValue: "500"
              metricStatPeriod: "60"
              metricStatType: Sum
              awsRegion: us-east-1
```

This scales the API based on the request count hitting the Application Load Balancer. When traffic exceeds 500 requests per target per minute, KEDA adds replicas.

### Fallback Configuration

Define fallback behavior when KEDA cannot retrieve metrics from the trigger source, ensuring your service maintains a safe replica count:

```yaml
services:
  worker:
    build: .
    command: bin/worker
    scale:
      count: 1-10
      keda:
        fallback:
          failureThreshold: 3
          replicas: 5
        triggers:
          - type: aws-sqs-queue
            metadata:
              queueURL: https://sqs.us-east-1.amazonaws.com/123456789/jobs
              queueLength: "5"
              awsRegion: "us-east-1"
```

If KEDA fails to read metrics after 3 consecutive attempts, the service scales to the fallback replica count of 5.

## AWS Authentication

On AWS racks, KEDA automatically uses the rack's IAM role for authentication with AWS services (SQS, CloudWatch, etc.) via IRSA. No additional authentication configuration is required for AWS-native triggers.

## See Also

- [Autoscaling](/configuration/scaling/autoscaling) for standard CPU/memory autoscaling
- [VPA](/configuration/scaling/vpa) for automatic resource right-sizing
- [keda_enable](/configuration/rack-parameters/aws/keda_enable) rack parameter
- [KEDA Scalers documentation](https://keda.sh/docs/2.19/scalers/) for all available trigger types and configuration
