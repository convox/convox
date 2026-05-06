---
title: "in_cluster_grafana_admin_password"
slug: in_cluster_grafana_admin_password
url: /configuration/rack-parameters/aws/in_cluster_grafana_admin_password
---

# in_cluster_grafana_admin_password

## Description
Admin password for the in-cluster Grafana sub-chart bundled with the paid `kube-prometheus-stack` Helm release. Required when [`enable_in_cluster_grafana`](/configuration/rack-parameters/aws/enable_in_cluster_grafana) is set to `true`. The chart refuses to render an unauthenticated Grafana, so this parameter is a hard prerequisite for the bundled Grafana flow.

The parameter value is treated as sensitive: stored only in a Kubernetes Secret on the rack, masked as `**********` in `convox rack params` TTY output (override with `--reveal`), never logged in plaintext, never serialized into rack deploy-spec annotations.

## Default Value
The default is `""` (empty string). When empty, the in-cluster Grafana sub-chart cannot deploy — `enable_in_cluster_grafana=true` with no password set is a configuration error.

## Use Cases
- **Initial bundled-Grafana enablement**: Set in the same `convox rack params set` call as `enable_in_cluster_grafana=true`.
- **Periodic password rotation**: Update without disabling; the chart picks up the new value on its next reconciliation cycle.
- **Password rotation after personnel changes**: Set a new value, then notify the engineers who need access. The previous password is invalidated immediately on the next rack reconciliation.

## Setting Parameters
To set or rotate the admin password:
```bash
$ convox rack params set in_cluster_grafana_admin_password=<your-strong-password> -r rackName
Updating parameters... OK
```

To clear (disables the bundled Grafana on next reconciliation; the Grafana pod fails to start without an admin password):
```bash
$ convox rack params set in_cluster_grafana_admin_password='' -r rackName
Updating parameters... OK
```

To set both `enable_in_cluster_grafana` and the password atomically:
```bash
$ convox rack params set enable_in_cluster_grafana=true in_cluster_grafana_admin_password=<your-strong-password> -r rackName
Setting parameters... OK
```

## Additional Information
- This parameter is AWS-only at this time. GCP, Azure, DigitalOcean, and Equinix Metal racks ship parallel Grafana integrations in subsequent releases.
- The value is treated as sensitive: stored as a Kubernetes Secret (not a ConfigMap), masked in `convox rack params` output (TTY) unless `--reveal` is passed, never logged in plaintext, never serialized into rack deploy-spec annotations.
- Pipe output (e.g. `convox rack params | grep in_cluster_grafana`) bypasses the TTY mask — the value is rendered verbatim. Use `--reveal` consciously when scripting around this parameter.
- The Grafana admin user is `admin` (Grafana's chart default). Use this password to log in.
- For zero-friction read-only access without an admin login, set `GF_AUTH_ANONYMOUS_ENABLED=true` + `GF_AUTH_ANONYMOUS_ORG_ROLE=Viewer` + `GF_AUTH_DISABLE_LOGIN_FORM=true` on the bundled Grafana via the Helm sub-chart's environment override (advanced; out of standard rack-param scope).

## Related Parameters
- [enable_in_cluster_grafana](/configuration/rack-parameters/aws/enable_in_cluster_grafana): The toggle that this password gates. Requires both to be set together.
- [grafana_url](/configuration/rack-parameters/aws/grafana_url): Set to the in-cluster Grafana's service URL so the Console GPU view's deep-link button takes you to the bundled Grafana.

## Version Requirements
This parameter requires at least Convox rack version `3.24.6`.
