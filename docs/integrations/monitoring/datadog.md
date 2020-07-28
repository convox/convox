# Datadog

You can add operational visibility to your [Rack](../../reference/primitives/rack) with Datadog.

## Configure Kubectl to Point at Your Rack

Convox allows you to securely connect your Kubectl to your Convox created Kubernetes cluster by exporting a [Kubeconfig](../../reference/cli/rack#rack-kubeconfig) that will connect you to a Kubernetes API Proxy running inside your Rack. This allows you to use Kubectl without directly exposing the credentials for your Kubernetes cluster. For example if your Rack is called `myrack` you could point your local Kubectl to your Rack cluster as follows

```
$ convox rack kubeconfig /tmp/myrack-config
$ export KUBECONTROL=/tmp/myrack-config
```

This will export the proxy configuration to a temporary file and then point your local Kubectl environment at that location so you can connect to your Rack's cluster. You will need to perform this step before you can execute any Kubectl commands against your cluster.

## Deploy the Datadog Agent

Once you have Kubectl pointing at your Rack you can deploy the datadog agent as a Kubernetes Daemonset. The following is based on the [DataDog Documentation](https://docs.datadoghq.com/agent/kubernetes/?tab=daemonset) so please refer back there for any specific tweaks you may want to make.

## Configure Agent Permissions

The following commands will create the necessary roles in your cluster for the DataDog Agent to monitor your cluster and your [Apps](../../reference/primitives/apps.md)

```
$ kubectl apply -f "https://raw.githubusercontent.com/DataDog/datadog-agent/master/Dockerfiles/manifests/rbac/clusterrole.yaml"

$ kubectl apply -f "https://raw.githubusercontent.com/DataDog/datadog-agent/master/Dockerfiles/manifests/rbac/serviceaccount.yaml"

$ kubectl apply -f "https://raw.githubusercontent.com/DataDog/datadog-agent/master/Dockerfiles/manifests/rbac/clusterrolebinding.yaml"

```

## Store Your API Key as a Kubernetes Secret

Retrieve your [API Key](https://app.datadoghq.com/account/settings#api) from the DataDog console and store it as a secret in your cluster by replacing `<DATADOG_API_KEY>` with your API key and running the following command.

```
$ kubectl create secret generic datadog-secret --from-literal api-key="<DATADOG_API_KEY>" --namespace="default"
```

You will then need to grab the Base64 encoded version of your API key to put in the DataDog Agent configuration file. You can retrieve the Base64 encoded value by running

`$ kubectl get secret datadog-secret -o yaml`

and grabbing the value for `api-key`

```
$ kubectl get secret datadog-secret -o yaml
apiVersion: v1
data:
  api-key: [BASE64 Encoded Key]
kind: Secret
metadata:
  creationTimestamp: "2020-07-27T20:42:29Z"
  name: datadog-secret
  namespace: default
  resourceVersion: "8898"
  selfLink: /api/v1/namespaces/default/secrets/datadog-secret
  uid: XXXXX-XXXXXX-XXXXX-XXXXXX
type: Opaque

```

## Create a DataDog Agent Manifest

DataDog has several example manifests
- [Manifest with Logs, APM, process, metrics collection enabled.](https://docs.datadoghq.com/resources/yaml/datadog-agent-all-features.yaml)
- [Manifest with Logs, APM, and metrics collection enabled.](https://docs.datadoghq.com/resources/yaml/datadog-agent-logs-apm.yaml)
- [Manifest with Logs and metrics collection enabled.](https://docs.datadoghq.com/resources/yaml/datadog-agent-logs.yaml)
- [Manifest with APM and metrics collection enabled.](https://docs.datadoghq.com/resources/yaml/datadog-agent-apm.yaml)
- [Manifest with Network Performance Monitoring enabled](https://docs.datadoghq.com/resources/yaml/datadog-agent-npm.yaml)
- [Vanilla manifest with just metrics collection enabled.](https://docs.datadoghq.com/resources/yaml/datadog-agent-vanilla.yaml)

Whichever one of these examples you use as your starting point, you will need to insert the Base64 encoded key from the previous step in the section at the top where it says:

```
# Source: datadog/templates/secrets.yaml
# API Key
apiVersion: v1
kind: Secret
metadata:
  name: datadog-agent
  labels: {}
type: Opaque
data:
  api-key: PUT_YOUR_BASE64_ENCODED_API_KEY_HERE
```

We also recommend adding the following to the environment section of the Daemonset spec:

```
- name: DD_CONTAINER_EXCLUDE
  value: "name:datadog-agent"
```
to remove extra noise from the DataDog Agent itself.

## Create the Daemonset

If you save your customized Agent Manifest as a file called `datadog-agent.yaml` you can then create the daemonset by running

```
$ kubectl apply -f datadog-agent.yaml
```

You can verify the daemonset by running

```
$ kubectl get daemonset
```

Once your `DESIRED` `CURRENT` and `READY` counts are all equal your Agents should be up and running. To make any changes to your Agent configuration simply modify your manifest and repeat the steps above.

For further customization and troubleshooting please refer to the [DataDog Docs](https://docs.datadoghq.com/agent/kubernetes/?tab=daemonset)

## Metrics and Traces

In order to use Datadog's APM, Distributed Tracing, or Runtime Metrics you will need
to connect to the Datadog agent.

The agent configuration above will be listening to `8125/udp` and `8126/tcp` on the instance
IP address. This IP address will be available to your [Processes](../../reference/primitives/app/process.md)
in the `INSTANCE_IP` environment variable.
