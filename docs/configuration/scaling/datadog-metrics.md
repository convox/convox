---
title: "Datadog Metrics Autoscaling"
slug: datadog-metrics
url: /configuration/scaling/datadog-metrics
---
# Datadog Metrics Autoscaling

You can autoscale Convox services based on Datadog metrics, allowing scaling decisions driven by business-level signals like request rates or queue depths rather than just CPU and memory utilization.

Convox passes your Datadog metric configuration directly into Kubernetes HPA external metric targets. This requires the Datadog Cluster Agent to be running in your cluster as an external metrics provider.

> **KEDA and Datadog**: If you use KEDA, you can scale on Datadog metrics via the [KEDA Datadog scaler](https://keda.sh/docs/2.19/scalers/datadog/) instead of HPA external metrics. This requires the Datadog Cluster Agent with external metrics enabled. When KEDA is configured for a service, Convox uses the KEDA ScaledObject rather than a native HPA, so HPA-based external metric targets in `scale.targets.external` are not applied. See [KEDA Autoscaling](/configuration/scaling/keda) for details.

## Prerequisites

- **A Convox Rack.** Check status with `convox rack -r rackNAME`.
- **Datadog Agent and Cluster Agent installed.** Follow the [Datadog integration guide](/integrations/monitoring) to deploy the agent to your rack. Use the `datadog-agent-all-features.yaml` manifest to ensure the Cluster Agent is included.

Verify the Cluster Agent is running:

```bash
$ kubectl get pods | grep cluster-agent
datadog-cluster-agent-b5fd4b7f5-tmkql   1/1     Running   0          16m
```

If you do not see a running `datadog-cluster-agent` pod, revisit the [Datadog integration guide](/integrations/monitoring) and ensure you used the `datadog-agent-all-features.yaml` manifest.

## Configure the External Metrics Provider

Follow the [Datadog external metrics documentation](https://docs.datadoghq.com/containers/guide/cluster_agent_autoscaling_metrics/?tab=daemonset#register-the-external-metrics-provider-service) to register the Cluster Agent as an external metrics provider, then follow the [DatadogMetric setup steps](https://docs.datadoghq.com/containers/guide/cluster_agent_autoscaling_metrics/?tab=daemonset#datadog-cluster-agent).

## Create a DatadogMetric

Create a `DatadogMetric` custom resource that defines the query Datadog will evaluate:

```yaml
apiVersion: datadoghq.com/v1alpha1
kind: DatadogMetric
metadata:
  name: <your_datadogmetric_name>
spec:
  query: <your_custom_query>
```

Deploy it with `kubectl apply -f your-custom-metric-file.yaml`

If your application sends custom metrics to Datadog via StatsD, you also need a Kubernetes Service to route traffic to the Datadog Agent:

```yaml
apiVersion: v1
kind: Service
metadata:
  name: dd-agent
spec:
  selector:
    app: datadog
  ports:
    - protocol: UDP
      port: 8125
      targetPort: 8125
```

Deploy with `kubectl apply -f dd-agent-service.yaml`

## Configure convox.yml

Reference the DatadogMetric in your service's `scale.targets.external` section. The naming convention is `datadogmetric@<namespace>:<datadogmetric_name>`. The `datadogmetric@<namespace>:<metric-name>` format references a DatadogMetric custom resource. The namespace is typically `default` unless your app uses a different Kubernetes namespace.

Convox supports two target types:

| Field | Description |
|-------|-------------|
| **averageValue** | Target value per replica (per-pod target). Kubernetes divides the metric by replica count. Use this when scaling should be based on per-replica load — for example, keeping request rate per pod at a steady level. |
| **value** | Absolute target value. Scaling is based on the raw metric regardless of replica count. Use this when the total metric value should drive scaling — for example, a single queue depth that all replicas consume from. |

## Example: Scaling on Page Views

In this example we will start with a [nodejs app](https://github.com/convox-examples/nodejs-dd-autoscale) that has been configured to scale with Datadog to demonstrate how to setup scaling with this method.

Create a `DatadogMetric` for page views:
```bash
$ cat page-views-metrics.yaml
apiVersion: datadoghq.com/v1alpha1
kind: DatadogMetric
metadata:
  name: page-views-metrics
spec:
  query: avg:page.views{*}.as_count()
```
Deploy with `kubectl apply -f page-views-metrics.yaml`

Configure the service in `convox.yml` to scale based on this metric. Set `DD_AGENT_HOST` so the app can push metrics to the Datadog Agent:

```yaml
environment:
  - PORT=3000
services:
  web:
    build: .
    port: 3000
    environment:
      - DD_AGENT_HOST=dd-agent.default.svc.cluster.local
    scale:
      count: 1-3       # min 1, max 3 replicas
      targets:
        external:
          - name: "datadogmetric@default:page-views-metrics"  # references the DatadogMetric CR in the "default" namespace
            averageValue: 5  # target value per pod — HPA will scale to maintain this average across all pods
```
[Configured Nodejs app](https://github.com/convox-examples/nodejs-dd-autoscale)

You can now deploy the application with `convox apps create -a nodejs` then `convox deploy -a nodejs`

Once deployed you can check the replica count and deployed services:
```bash
$ convox ps -a nodejs
ID                    SERVICE  STATUS   RELEASE      STARTED        COMMAND
web-5bc58fb455-psd9h  web      running  RMOGLKGFMOW  7 minutes ago

$ convox services -a nodejs
SERVICE  DOMAIN                                    PORTS
web      web.nodejs.1e5717e816e99649.convox.cloud  443:3000
```

You can simulate some views to make the application scale via:
`$ while true; do curl https://<YOUR_SERVICE_DOMAIN>/; sleep 0.2; done`

You can then see that the application is beginning to scale up:
```bash
$ convox ps -a nodejs
ID                    SERVICE  STATUS   RELEASE      STARTED         COMMAND
web-5675cccf75-chmcc  web      running  RAZUSIKBQGX  25 seconds ago
web-5675cccf75-dm6kb  web      running  RAZUSIKBQGX  6 minutes ago
```

When you end the view simulation it will scale down again:
```bash
$ convox ps -a nodejs
ID                    SERVICE  STATUS   RELEASE      STARTED         COMMAND
web-5675cccf75-dm6kb  web      running  RAZUSIKBQGX  11 minutes ago
```

## See Also

- [Autoscaling](/configuration/scaling/autoscaling) for CPU/memory-based scaling
- [KEDA Autoscaling](/configuration/scaling/keda) for event-driven autoscaling
- [Datadog Integration](/integrations/monitoring) for setting up Datadog
