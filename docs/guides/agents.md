# Agents

Agents are specialized services that run a single process on each cluster instance.

## Configuration

You can define any given Service as an Agent by setting its `agent` attribute to `true`

    services:
      telemetry:
        agent: true

### Communicating with an Agent

Often it is useful for other Services to communicate with an agent running on its instance.

You can declare ports that will be available to communicate with an agent using the `ports` attribute:

    services:
      telemetry:
        agent: true
        ports:
          - 8125/udp
          - 8126

> Agents will listen on the IP address of the underlying instance. This means that you can not deploy
> two agents on the same Rack that listen on the same port.

Each Service will have the IP of its instance available in the `INSTANCE_IP` environment variable.

In the example above, other Services can communicate with the `telemetry` agent running on its instance
using the following endpoints:

* `udp://$INSTANCE_IP:8125`
* `tcp://$INSTANCE_IP:8126`