---
title: "elasticsearch_disk"
description: "The elasticsearch_disk Azure rack parameter sizes each Elasticsearch log volume, which limits how much log history the Rack keeps, defaulting to 5Gi."
slug: elasticsearch_disk
url: /configuration/rack-parameters/azure/elasticsearch_disk
---

# elasticsearch_disk

## Description

The `elasticsearch_disk` parameter sets the size of each of the Rack's Elasticsearch log volumes. Elasticsearch stores the logs that `convox logs` without `--service` and `convox rack logs` read. The Rack runs two Elasticsearch Pods, each with its own persistent volume of this size. The volumes are separate Azure managed disks, not node disk, so [node_disk](/configuration/rack-parameters/azure/node_disk) does not change them.

When a volume reaches 80% full, the Rack deletes the oldest day of logs, so the size of the volumes limits how much log history the Rack keeps. See [Log Retention](/configuration/logging#log-retention).

## Default Value

The default value is `5Gi`, the size every Rack's volumes are created at.

## Accepted Values

A whole number of Gi, at least `5Gi`, such as `5Gi`, `20Gi` or `100Gi`. The CLI rejects any other value:

```text
ERROR: param 'elasticsearch_disk' must be a whole number of Gi and at least 5Gi, such as 20Gi
```

The volumes can grow but never shrink, so the CLI also refuses a value below the one `convox rack params` currently lists:

```text
ERROR: param 'elasticsearch_disk' cannot be below the current 10Gi: the volumes can grow but not shrink (use --force to set it anyway)
```

A lower value set with `--force` is stored and leaves the volumes at their current size.

The parameter cannot be cleared.

## Use Cases

- **Longer log history**: keep more days of logs on a busy Rack.
- **Room for one day of logs**: the Rack never deletes the current UTC day, so a Rack that writes close to a full volume in one day needs larger volumes.

## Setting the Parameter

```bash
$ convox rack params set elasticsearch_disk=20Gi -r rackName
Updating parameters... OK
```

The change restarts the Rack API, which then requests the larger size for both volumes. `convox rack logs` shows a line like this for each volume:

```text
ns=elastic at=expand claim=elasticsearch-data-elasticsearch-0 from=5Gi to=20Gi
```

What happens next depends on the [node_type](/configuration/rack-parameters/azure/node_type) of the nodes the Elasticsearch Pods run on. Some VM sizes, including the default `Standard_D2_v3`, cannot grow a disk while it is attached:

| Node size | Result |
|---|---|
| A size that can grow an attached disk | The volumes grow while Elasticsearch keeps running |
| A size that cannot, including `Standard_D2_v3` | About 10 minutes after the Rack requests the larger size, it stops Elasticsearch so Azure can detach and grow the disks, then starts it again, usually within 15 minutes |

While Elasticsearch is stopped:

- `convox logs` without `--service` and `convox rack logs` print an error instead of logs.
- `convox logs --service <name>` keeps working, because it reads Pod logs directly.
- Fluentd holds the lines written in the meantime and delivers them once Elasticsearch is back.

The stop and the start appear in `convox rack logs` once Elasticsearch is back:

```text
ns=elastic at=expand stopping=elasticsearch replicas=2
ns=elastic at=expand starting=elasticsearch replicas=2
```

The Rack makes one attempt per value. If the disks have not grown 30 minutes after the stop, it starts Elasticsearch at the old size, and the start line ends with `warning="volumes did not grow while detached"`. To try again, set a larger value.

Each of the two volumes is billed as a separate disk of this size.

## Viewing Current Configuration

```bash
$ convox rack params -r rackName
```

The parameter is listed only once it is set. `convox rack params -g storage -r rackName` shows it with the other storage parameters.

## Additional Information

- A value given on `convox rack install` creates the volumes at `5Gi`, and the Rack grows them to the value once it is running. On a node size that cannot grow an attached disk, that includes the stop and start described above.
- Requires Rack version `3.25.11` or later, and CLI version `3.25.11` or later to set it. An older CLI rejects the parameter on Azure with `unknown parameter 'elasticsearch_disk' for azure provider`.
- Available on GCP, Azure, DigitalOcean and metal Racks. See [GCP](/configuration/rack-parameters/gcp/elasticsearch_disk) and [DigitalOcean](/configuration/rack-parameters/do/elasticsearch_disk). On metal Racks, set it through the CLI.
- Downgrading below `3.25.11` removes the parameter with `NOTICE: removing parameters not supported by version <version>: elasticsearch_disk` on stderr. The volumes keep their size. Logs written while the Rack ran `3.25.11` are hidden from `convox logs` until the Rack is updated to `3.25.11` or later again; nothing is deleted. A Rack managed through the Console keeps the stored value and applies it again on the upgrade. A self-managed Rack drops the value, and `convox rack params` does not list it until it is set again.

## See Also

- [Log Retention](/configuration/logging#log-retention) for how the Rack deletes old logs
- [Elasticsearch on GCP, Azure, DigitalOcean and Metal Racks](/configuration/logging#elasticsearch-on-gcp-azure-digitalocean-and-metal-racks) for how logs are stored and read
- [logs](/reference/cli/logs) for reading App logs
- [node_type](/configuration/rack-parameters/azure/node_type) for the VM size of the Rack's nodes
- [fluentd_memory](/configuration/rack-parameters/azure/fluentd_memory) for the memory of the Fluentd log collector
- [syslog](/configuration/rack-parameters/azure/syslog) for forwarding logs to an external service for long-term storage
- [node_disk](/configuration/rack-parameters/azure/node_disk) for the size of each node's OS disk
