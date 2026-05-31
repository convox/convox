---
title: "keda_enable"
description: "The keda_enable AWS rack parameter installs KEDA on the rack for event-driven autoscaling on queues, databases, and custom metrics, defaulting to false."
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
Enabling KEDA installs the KEDA operator and its components in the cluster. Once enabled, services can use the `scale.keda` or `scale.autoscale` sections in their `convox.yml` to define autoscaling triggers.

The Console **Service > Scaling** page surfaces a status card describing KEDA state on the rack. Whether Range mode (min/max replica editing) is available depends on the service having any autoscaler at all (manifest-declared OR Console-driven). Native HPA backs CPU/memory autoscale on every rack regardless of `keda_enable`; KEDA only gates the event-driven trigger types (GPU utilization, queue depth, custom Prometheus, SQS, etc.).

The Console **Autoscale Triggers Override** surface (3.24.6+) also reads this parameter to gate the KEDA-only trigger types (GPU utilization, queue depth) in the Enable dialog. CPU and memory triggers use native Kubernetes HPA and remain available on every rack regardless of `keda_enable`. See [Autoscale Triggers Override](/console/autoscale-triggers) for details.

See [KEDA Autoscaling](/configuration/scaling/keda) for service configuration details.
