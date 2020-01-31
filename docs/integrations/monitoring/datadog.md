# Datadog

You can add operational visibility to your Convox environments with Datadog.

## Sign up for Datadog

If you don’t have an account already, [sign up for Datadog](https://app.datadoghq.com/signup). You’ll need an API key that lets you send data from Convox to the Datadog service.

## Deploy the Datadog Agent

You can deploy the datadog agent as a Convox app with a very simple `convox.yml` manifest:

```yml
services:
  datadog:
    agent: true
    ports:
      - 8125/udp
      - 8126/tcp
    image: datadog/agent:latest
    environment:
      - DD_API_KEY
      - DD_APM_ENABLED=true
    privileged: true
    scale:
      cpu: 128
      memory: 128
    volumes:
      - /sys/fs/cgroup/:/host/sys/fs/cgroup/
      - /proc/:/host/proc/
      - /var/run/docker.sock:/var/run/docker.sock
```

### Application Metrics

Each [Process](../reference/app/primitives/process.md) will have the IP address of its
[Instance](../reference/app/primitives/instance.md) available in the `INSTANCE_IP` environment variable.

To forward application metrics to Datadog you'll need the host IP address. You can get it by simply referencing `$INSTANCE_IP`.
