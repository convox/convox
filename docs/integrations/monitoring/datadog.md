# Datadog

You can add operational visibility to your [Rack](../reference/primitives/rack) with Datadog.

## Deploy the Datadog Agent

You can deploy the datadog agent as a Convox [App](../reference/primitives/app) with
a simple `convox.yml`:

    services:
      datadog:
        agent: true
        environment:
          - DD_API_KEY
          - DD_APM_ENABLED=true
        image: datadog/agent:latest
        ports:
          - 8125/udp
          - 8126/tcp
        privileged: true
        scale:
          cpu: 128
          memory: 128
        volumes:
          - /proc/:/host/proc/
          - /sys/fs/cgroup/:/host/sys/fs/cgroup/
          - /var/run/docker.sock:/var/run/docker.sock

> Make sure to set the `DD_API_KEY` environment variable to your Datadog API key.

## Metrics and Traces

In order to use Datadog's APM, Distributed Tracing, or Runtime Metrics you will need
to connect to the Datadog agent.

The agent configuration above will be listening to `8125/udp` and `8126/tcp` on the instance
IP address. This IP address will be available to your [Processes](../../reference/primitives/app/process.md)
in the `INSTANCE_IP` environment variable.
