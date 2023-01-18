---
title: "Scaling"
draft: false
slug: Scaling
url: /deployment/scaling
---
# Scaling

Convox allows you to easily scale any [Service](/reference/primitives/app/service) on the following dimensions:

- Horizontal concurrency (number of [Processes](/reference/primitives/app/process))
- CPU allocation (in CPU units where 1000 units is one full CPU)
- Memory allocation (in MB)

## Initial Defaults

You can specify the scale for any [Service](/reference/primitives/app/service) in your [convox.yml](/configuration/convox-yml)
```html
    services:
      web:
        scale:
          count: 2
          cpu: 250
          memory: 512
```
> If you specify a static `count` it will only be used on first deploy. Subsequent changes must be made using the `convox` CLI.

## Manual Scaling

### Determine Current Scale
```html
    $ convox scale
    NAME  DESIRED  RUNNING  CPU  MEMORY
    web   2        2        250  512
```
### Scaling Count Horizontally
```html
    $ convox scale web --count=3
    Scaling web...
    2020-01-01T00:00:00Z system/k8s/web Scaled up replica set web-65f45567d to 2
    2020-01-01T00:00:00Z system/k8s/web-65f45567d Created pod: web-65f45567d-c7sdw
    2020-01-01T00:00:00Z system/k8s/web-65f45567d-c7sdw Successfully assigned dev-convox/web-65f45567d-c7sdw to node
    2020-01-01T00:00:00Z system/k8s/web-65f45567d-c7sdw Container image "registry.dev.convox/convox:web.BABCDEFGHI" already present on machine
    2020-01-01T00:00:00Z system/k8s/web-65f45567d-c7sdw Created container main
    2020-01-01T00:00:00Z system/k8s/web-65f45567d-c7sdw Started container main
    OK
```
> Changes to `cpu` or `memory` should be done in your `convox.yml`, and a new release of your app deployed.

## Autoscaling

To use autoscaling you must specify a range for allowable [Process](/reference/primitives/app/process) count and
target values for CPU and Memory utilization (in percent):
```html
    service:
      web:
        scale:
          count: 1-10
          targets:
            cpu: 70
            memory: 90
```
The number of [Processes](/reference/primitives/app/process) will be continually adjusted to maintain your target metrics.

You must consider that the targets for CPU and Memory use the service replicas limits to calculate the utilization percentage. So if you set the target for CPU as `70` and have two replicas, it will trigger the auto-scale only if the utilization percentage sum divided by the replica's count is bigger than 70%. The desired replicas will be calculated to satisfy the percentage. Being the `currentMetricValue` computed by taking the average of the given metric across all service replicas.

```html
desiredReplicas = ceil[currentReplicas * ( currentMetricValue / desiredMetricValue )]
```

### Autoscaling With Custom Metrics

#### Datadog

*To use Datadog metrics for autoscaling your rack must be updated to [version 3.9.1](https://github.com/convox/convox/releases/tag/3.9.1) or later.  You can find your rack's version by running `convox rack -r rackNAME`. If you are on an older version, please [update your rack](https://docs.convox.com/management/cli-rack-management/) to use this feature.


To autoscale based on Datadog metrics you must first have Datadog and the Datadog Cluster Agent installed. If you used the `datadog-agent-all-features.yaml` when [configuring Datadog](/integrations/monitoring/datadog/) then you will already have the Datadog Cluster Agent installed.

You can check your cluster's pods to see what is installed:
* If you installed into a custom namespace you will have to add the appropriate `-n NAMESPACE_NAME` option to the end of the command
```html 
    $ kubectl get pods                                                          
    NAME                                    READY   STATUS    RESTARTS   AGE 
    datadog-bjw2m                           5/5     Running   0          16m 
    datadog-cluster-agent-b5fd4b7f5-tmkql   1/1     Running   0          16m 
    datadog-j9c9t                           5/5     Running   0          15m 
    datadog-pnrln                           5/5     Running   0          16m 
    datadog-vdzc5                           5/5     Running   0          16m 
``` 

If your installation does not have an active datadog-cluster-agent pod you can install the Datadog Cluster Agent by following the instructions from the [Datadog Documentation](https://docs.datadoghq.com/containers/cluster_agent/setup/?tab=daemonset). 


Once the Datadog Cluster Agent is setup follow this [Datadog Documentation](https://docs.datadoghq.com/containers/guide/cluster_agent_autoscaling_metrics/?tab=daemonset#register-the-external-metrics-provider-service) to configure your Datadog Cluster Agent with an external metrics provider service.


Next to setup DatadogMetric follow these steps from the [Datadog Documentation](https://docs.datadoghq.com/containers/guide/cluster_agent_autoscaling_metrics/?tab=daemonset#datadog-cluster-agent).

You can now create scaling manifests based on Datadog's provided template:
```html 
apiVersion: datadoghq.com/v1alpha1
kind: DatadogMetric
metadata:
  name: <your_datadogmetric_name>
spec:
  query: <your_custom_query>
``` 
You can deploy these metrics with `kubectl apply -f your-custom-metric-file.yaml`

You will finally then need to deploy a service for your apps to push metrics to Datadog.  You can use this example or write your own based on your own custom configurations:
```html 
$ cat dd-agent-service.yaml
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
Deploy the service with `kubectl apply -f dd-agent-service.yaml`


In this example we will start with a [nodejs app](https://github.com/convox-examples/nodejs-dd-autoscale) that has been configured to scale with Datadog to demonstrate how to setup scaling with this method. 

Based on the provided template we can make a basic page views metric for the node application: 
```html 
$ cat page-views-metrics.yaml
apiVersion: datadoghq.com/v1alpha1 
kind: DatadogMetric 
metadata: 
  name: page-views-metrics 
spec: 
  query: avg:page.views{*}.as_count() 
```
Then add the metric to your cluster via `kubectl apply -f page-views-metrics.yaml`


Now we are going to look at the configuration of the `convox.yaml` from our nodejs app to autoscale based on this newly created metric.

 We will specify the `datadogmetric` object name in the `scale.targets[].external section`. The `datadogmetric` object name convention to use in `scale.targets[].external` section is : `datadogmetric@<namespace>:<datadogmetric_name>`

We set the env var `DD_AGENT_HOST` to point at the dd-agent service we previously made to push information to Datadog.

```html 
environment:
  - PORT=3000
services:
  web:
    build: .
    port: 3000
    environment:
      - DD_AGENT_HOST=dd-agent.default.svc.cluster.local
    scale:
      count: 1-3
      targets:
        external:
          - name: "datadogmetric@default:page-views-metrics"
            averageValue: 5
``` 
[Configured Nodejs app](https://github.com/convox-examples/nodejs-dd-autoscale)

You can now deploy the application with `convox apps create` then `convox deploy`

Once deployed you can check the replica count and deployed services:
```html
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
```html
$ convox ps -a nodejs                
ID                    SERVICE  STATUS   RELEASE      STARTED         COMMAND
web-5675cccf75-chmcc  web      running  RAZUSIKBQGX  25 seconds ago  
web-5675cccf75-dm6kb  web      running  RAZUSIKBQGX  6 minutes ago   
```

When you end the view simulation it will scale down again:
```html
$ convox ps -a nodejs
ID                    SERVICE  STATUS   RELEASE      STARTED         COMMAND
web-5675cccf75-dm6kb  web      running  RAZUSIKBQGX  11 minutes ago  
```