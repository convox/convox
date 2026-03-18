---
title: "keda_enable"
slug: keda_enable
url: /configuration/rack-parameters/aws/keda_enable
---

# keda_enable

## Description
The `keda_enable` parameter installs KEDA (Kubernetes Event-Driven Autoscaling) on the rack. KEDA enables event-driven autoscaling for services, allowing them to scale based on external event sources such as message queues, databases, and custom metrics.

## Default Value
The default value for `keda_enable` is `false`.

## Use Cases
- **Event-Driven Scaling**: Scale services based on AWS SQS queue depth, Kafka lag, or other event sources.
- **Scale to Zero**: Reduce costs by scaling idle services to zero replicas when no events are pending.
- **Custom Metrics**: Autoscale on application-specific metrics from Prometheus, Datadog, or other monitoring sources.

## Setting Parameters
To enable KEDA on your rack, use the following command:
```bash
$ convox rack params set keda_enable=true -r rackName
Setting parameters... OK
```

## Additional Information
Enabling KEDA installs the KEDA operator and its components in the cluster. Once enabled, services can use the `scale.keda` section in their `convox.yml` to define event-driven scaling triggers.

See [KEDA Autoscaling](/configuration/scaling/keda) for service configuration details.
