---
title: "Monitoring"
slug: monitoring
url: /integrations/monitoring
---
# Monitoring Integrations

Convox integrates with third-party monitoring platforms to provide deep operational visibility into your [Rack](/reference/primitives/rack) and applications. These integrations complement the [built-in monitoring and alerting](/configuration/monitoring) that Convox provides natively.

Monitoring integrations are installed by deploying agents directly into your Rack's Kubernetes cluster using `kubectl`. Once deployed, agents collect metrics, traces, and logs from all nodes and forward them to the monitoring platform.

## Datadog

You can add operational visibility to your Rack with Datadog.

### Configure kubectl to Point at Your Rack

Convox allows you to securely connect your `kubectl` to your Convox created Kubernetes cluster by exporting a [kubeconfig](/reference/cli/rack#rack-kubeconfig) that will connect you to a Kubernetes API Proxy running inside your Rack. This allows you to use `kubectl` without directly exposing the credentials for your Kubernetes cluster. For example, if your Rack is called `myrack` you could point your local `kubectl` to your Rack cluster as follows.

```bash
$ convox rack kubeconfig > /tmp/myrack-config
$ export KUBECONFIG=/tmp/myrack-config
```

This will export the proxy configuration to a temporary file and then point your local `kubectl` environment at that location so you can connect to your Rack's cluster. You will need to perform this step before you can execute any `kubectl` commands against your cluster.

### Deploy the Datadog Agent

Once you have `kubectl` pointing at your Rack you can deploy the datadog agent as a Kubernetes Daemonset. The following is based on the [Datadog Daemonset Installation Instructions](https://docs.datadoghq.com/agent/kubernetes/?tab=daemonset) so please refer back there for any specific tweaks you may want to make.

We recommend that you use the manifest `datadog-agent-all-features.yaml` when applying the agent. This ensures you can enter and edit the desired variables in one manifest file.
If you prefer, you can install the agent using piecewise manifests.

During installation we recommend adding the following to the environment section of the Daemonset spec:

```text
- name: DD_CONTAINER_EXCLUDE
  value: "name:datadog-agent"
```
This will remove extra noise from the Datadog Agent itself.

### Verify Installation

You can verify the installation by running:

```bash
$ kubectl get daemonset
NAME      DESIRED   CURRENT   READY   UP-TO-DATE   AVAILABLE   NODE SELECTOR            AGE
datadog   4         4         4       4            4           kubernetes.io/os=linux   134m
$
$ kubectl get pods
NAME                                    READY   STATUS    RESTARTS   AGE
datadog-bjw2m                           5/5     Running   0          135m
datadog-cluster-agent-b5fd4b7f5-tmkql   1/1     Running   0          135m
datadog-j9c9t                           5/5     Running   0          134m
datadog-pnrln                           5/5     Running   0          135m
datadog-vdzc5                           5/5     Running   0          135m
```

Once your `DESIRED` `CURRENT` and `READY` counts are all equal your Agents should be up and running. To make any changes to your Agent configuration modify your manifest and repeat the steps.

For further customization and troubleshooting please refer to the [Datadog Daemonset Config Docs](https://docs.datadoghq.com/containers/kubernetes/configuration?tab=daemonset).

### Metrics and Traces

In order to use Datadog's APM, Distributed Tracing, or Runtime Metrics you will need
to connect to the Datadog agent.

The agent configuration above will be listening to `8125/udp` and `8126/tcp` on the instance
IP address. This IP address will be available to your [Processes](/reference/primitives/app/process)
in the `INSTANCE_IP` environment variable.

You can autoscale based on Datadog Metrics with a few [additional steps](/configuration/scaling/datadog-metrics).

## See Also

- [Monitoring and Alerting](/configuration/monitoring) for Convox's built-in monitoring capabilities
- [Observability](/configuration/observability) for an overview of logging and monitoring options
