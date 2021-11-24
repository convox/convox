# CLI Rack Management

## Updating a Rack

### Updating to the latest version

    $ convox rack update
    Updating rack... OK

### Pinning to a specific version

    $ convox rack update 3.0.0
    Updating rack... OK

## Managing Parameters

### Viewing current parameters

    $ convox rack params
    node_disk  20
    node_type  t3.small

### Setting parameters

    $ convox rack params set node_disk=30 node_type=c5.large
    Setting parameters... OK

## Available Parameters

The parameters available for your Rack depend on the underlying cloud provider.

### Amazon Web Services

| Name                  | Default       |
|-----------------------|---------------|
| `cidr`                | `10.1.0.0/16` |
| `node_disk`           | `20`          |
| `node_type`           | `t3.small`    |
| `region`              | `us-east-1`   |
| `high_availability` * | `true`        |

\* Parameter cannot be changed after rack creation

### Digital Ocean

| Name                  | Default       |
|-----------------------|---------------|
| `node_type`           | `s-2vcpu-4gb` |
| `region`              | `nyc3`        |
| `registry_disk`       | `50Gi`        |
| `high_availability` * | `true`        |

\* Parameter cannot be changed after rack creation

### Google Cloud

| Name        | Default         |
| ----------- | --------------- |
| `node_type` | `n1-standard-1` |

### Microsoft Azure

| Name        | Default          |
| ----------- | ---------------- |
| `node_type` | `Standard_D3_v3` |
| `region`    | `eastus`         |
