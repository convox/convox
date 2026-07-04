---
title: "instances"
description: "The convox instances command lists cluster instances and manages per-instance operations such as terminate, SSH key roll, and opening a shell over SSH."
slug: instances
url: /reference/cli/instances
---
# instances

## instances

List instances

### Usage
```bash
    convox instances
```
### Examples
```bash
    $ convox instances
    ID                          STATUS   STARTED       PS  CPU     MEM     PUBLIC          PRIVATE     TYPE
    ip-10-0-2-39.ec2.internal   running  2 months ago  3   18.75%  45.08%  34.207.218.250  10.0.2.39   t3.large
    ip-10-0-2-17.ec2.internal   running  2 months ago  2   18.75%  32.64%  52.208.102.198  10.0.2.17   t3.large
    ip-10-0-1-151.ec2.internal  running  2 months ago  3   21.88%  58.13%  52.160.141.135  10.0.1.151  m5.xlarge
    ip-10-0-3-45.ec2.internal   running  2 months ago  5   37.50%  77.72%  3.226.241.132   10.0.3.45   m5.xlarge
    ip-10-0-1-56.ec2.internal   running  2 months ago  5   50.00%  97.91%  52.144.245.183  10.0.1.56   c5.large
```
## instances terminate

Terminate an instance

### Usage
```bash
    convox instances terminate <instance_id>
```
### Examples
```bash
    $ convox instances terminate ip-10-0-2-39.ec2.internal
    Terminating instance... OK
```

See [Instance](/reference/primitives/rack/instance#termination-behavior) for details on the drain sequence, PodDisruptionBudget handling, and per-provider cloud VM reclamation.

## instances keyroll

Roll ssh key on instances

### Usage
```bash
    convox instances keyroll
```
This generates a private key and displays it as output. Save the generated private key and use it when connecting via SSH to an instance.
### Examples
```bash
    $ convox instances keyroll
    Rolling instance key... OK
    Updating parameters... OK
    Generated private key:
    -----BEGIN RSA PRIVATE KEY-----
    MIIE...
    -----END RSA PRIVATE KEY-----
```

## instances ssh

Run a shell on an instance

### Usage
```bash
    convox instances ssh <instance_id> --key <private_key_file>
```

### Flags

| Flag | Description |
| ---- | ----------- |
| `--key` | Path to private key file (from `instances keyroll`) |

### Examples
```bash
    $ convox instances ssh ip-10-1-80-201.ec2.internal --key ~/.ssh/rack/priv.pem
```

## See Also

- [key_pair_name](/configuration/rack-parameters/aws/key_pair_name) for configuring SSH access to cluster nodes
- [Instance](/reference/primitives/rack/instance) for termination behavior and stuck-node cleanup
