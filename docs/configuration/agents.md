# Agents

Agents are specialized [Services](../reference/primitives/app/service.md) that run a single
[Process](../reference/primitives/app/process.md) on each [Instance](../reference/primitives/app/instance.md).

## Configuration

You can define any [Service](../reference/primitives/app/service.md) as an Agent by setting
its `agent` attribute to `true`

    services:
      telemetry:
        agent: true

### Communicating with an Agent

Often it is useful for other [Processes](../reference/primitives/app/process.md) to communicate
with an Agent running on its [Instance](../reference/primitives/app/instance.md).

You can declare ports that will be available to communicate with an Agent using the `ports` attribute:

    services:
      telemetry:
        agent: true
        ports:
          - 8125/udp
          - 8126

> Agents will listen on the IP address of the underlying [Instance](../reference/primitives/app/instance.md).
> This means that you can not deploy two Agents on the same Rack that listen on the same port.

Each [Process](../reference/primitives/app/process.md) will have the IP address of its
[Instance](../reference/primitives/app/instance.md) available in the `INSTANCE_IP` environment variable.

In the example above, any [Process](../reference/primitives/app/service.md) on the same Rack can communicate
with the `telemetry` Agent running on its [Instance](../reference/primitives/app/instance.md) using the
following endpoints:

* `udp://$INSTANCE_IP:8125`
* `tcp://$INSTANCE_IP:8126`