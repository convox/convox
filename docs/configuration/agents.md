---
title: "Agents"
slug: agents
url: /configuration/agents
---
# Agents

Agents are specialized [Services](/reference/primitives/app/service) that run a single
[Process](/reference/primitives/app/process) on each [Instance](/reference/primitives/rack/instance).

## Defining an Agent

You can define any [Service](/reference/primitives/app/service) as an Agent by setting
its `agent` attribute to `true`
```yaml
services:
  telemetry:
    agent: true
```
### Communicating with an Agent

Often it is useful for other [Processes](/reference/primitives/app/process) to communicate
with an Agent running on its [Instance](/reference/primitives/rack/instance).

You can declare ports that will be available to communicate with an Agent using the `ports` attribute:
```yaml
services:
  telemetry:
    agent: true
    ports:
      - 8125/udp
      - 8126
```
> Agents will listen on the IP address of the underlying [Instance](/reference/primitives/rack/instance).
> This means that you cannot deploy two Agents on the same Rack that listen on the same port.

Each [Process](/reference/primitives/app/process) will have the IP address of its
[Instance](/reference/primitives/rack/instance) available in the `INSTANCE_IP` environment variable.

In the example above, any [Process](/reference/primitives/app/service) on the same Rack can communicate
with the `telemetry` Agent running on its [Instance](/reference/primitives/rack/instance) using the
following endpoints:

* `udp://$INSTANCE_IP:8125`
* `tcp://$INSTANCE_IP:8126`

> A process can only communicate with the agent running on the same node. Ensure the `INSTANCE_IP` corresponds to the node that the process is running on.
