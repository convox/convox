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
Enabling KEDA installs the KEDA operator and its components in the cluster. Once enabled, services can use the `scale.keda` or `scale.autoscale` sections in their `convox.yml` to define autoscaling triggers.

The Console **Service > Scaling** page reads this parameter to gate Range mode (min/max replica editing). When `keda_enable=false` the page surfaces the enable command in an empty-state card; when `keda_enable=true` and the service has a `scale.autoscale` block, Range mode becomes available for adjusting bounds without re-deploying. Services without a `scale.autoscale` (or `scale.keda`) block can still be scaled via Fixed count regardless of this parameter — KEDA only gates the autoscale path, not classic `count: 1-N` bounded scaling.

The Console **Autoscale Triggers Override** surface (3.24.6+) also reads this parameter to gate the KEDA-only trigger types (GPU utilization, queue depth). CPU and memory triggers use native Kubernetes HPA and work on every rack regardless of `keda_enable`. See [Autoscale Triggers Override](/console/autoscale-triggers) for details.

See [KEDA Autoscaling](/configuration/scaling/keda) for service configuration details.
