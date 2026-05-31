---
title: "grafana_dashboard_var_rack"
slug: grafana_dashboard_var_rack
url: /configuration/rack-parameters/aws/grafana_dashboard_var_rack
---

# grafana_dashboard_var_rack

## Description
The `grafana_dashboard_var_rack` parameter overrides the Grafana dashboard template variable name for the rack/cluster filter. The Console's "Open in Grafana" deep-link button substitutes this name into the URL it opens (e.g., `?var-rack=<rack>` becomes `?var-cluster_name=<rack>` when set to `cluster_name`).

Use this parameter when your imported Grafana dashboards use template variable names that differ from Convox's defaults. Most operators with custom dashboards or third-party imported dashboards will need at least one of the four `grafana_dashboard_var_*` overrides.

## Default Value
The default value is `rack`.

## Allowed Range
Letters, digits, and underscore only. Values containing any other character are rejected, since they would break URL substitution or Grafana template syntax.

## Use Cases
- **Imported dashboards using `cluster_name`**: Standard Kubernetes dashboards from Grafana Labs's gallery typically use `var-cluster_name` instead of `var-rack`.
- **Self-built dashboards using `var-cluster`**: Set `grafana_dashboard_var_rack=cluster` to match.

## Setting Parameters
To override to `cluster_name`:
```bash
$ convox rack params set grafana_dashboard_var_rack=cluster_name -r rackName
Setting parameters... OK
```

To revert to the default:
```bash
$ convox rack params set grafana_dashboard_var_rack=rack -r rackName
Setting parameters... OK
```

To clear the override (falls back to the canonical default `rack`):
```bash
$ convox rack params set grafana_dashboard_var_rack= -r rackName
Setting parameters... OK
```

## Operational Notes
- No restart is required. The next "Open in Grafana" deep-link uses the new value.
- The "Dashboard filter mismatch?" troubleshooting modal in the Console explains the four configurable var names and how to inspect Grafana's expected variable names.

## Related Parameters
- [grafana_dashboard_var_namespace](/configuration/rack-parameters/aws/grafana_dashboard_var_namespace): Companion override for the namespace filter.
- [grafana_dashboard_var_service](/configuration/rack-parameters/aws/grafana_dashboard_var_service): Companion override for the service filter.
- [grafana_dashboard_var_app](/configuration/rack-parameters/aws/grafana_dashboard_var_app): Companion override for the app filter.
- [grafana_url](/configuration/rack-parameters/aws/grafana_url): Base Grafana URL used by the deep-link button.

## Version Requirements
This parameter requires at least Convox rack version `3.24.6`.
