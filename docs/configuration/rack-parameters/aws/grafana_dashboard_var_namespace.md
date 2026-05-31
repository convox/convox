---
title: "grafana_dashboard_var_namespace"
description: "The grafana_dashboard_var_namespace AWS rack parameter overrides the Grafana dashboard template variable name for the namespace filter, defaulting to namespace."
slug: grafana_dashboard_var_namespace
url: /configuration/rack-parameters/aws/grafana_dashboard_var_namespace
---

# grafana_dashboard_var_namespace

## Description
The `grafana_dashboard_var_namespace` parameter overrides the Grafana dashboard template variable name for the namespace filter. The Console's "Open in Grafana" deep-link button substitutes this name into the URL it opens (e.g., `?var-namespace=<ns>` becomes `?var-k8s_namespace=<ns>` when set to `k8s_namespace`).

## Default Value
The default value is `namespace`.

## Allowed Range
Letters, digits, and underscore only. Values containing any other character are rejected, since they would break URL substitution or Grafana template syntax.

## Use Cases
- **Dashboards using `k8s_namespace`**: Some imported dashboards prefix Kubernetes-specific variables with `k8s_`.
- **Dashboards using `ns`**: Brevity-oriented dashboards often shorten `namespace` to `ns`.

## Setting Parameters
To override to `k8s_namespace`:
```bash
$ convox rack params set grafana_dashboard_var_namespace=k8s_namespace -r rackName
Setting parameters... OK
```

To revert to the default:
```bash
$ convox rack params set grafana_dashboard_var_namespace=namespace -r rackName
Setting parameters... OK
```

To clear the override (falls back to the canonical default `namespace`):
```bash
$ convox rack params set grafana_dashboard_var_namespace= -r rackName
Setting parameters... OK
```

## Operational Notes
- No restart is required. The next "Open in Grafana" deep-link uses the new value.
- The "Dashboard filter mismatch?" troubleshooting modal in the Console explains the four configurable var names and how to inspect Grafana's expected variable names.

## Related Parameters
- [grafana_dashboard_var_rack](/configuration/rack-parameters/aws/grafana_dashboard_var_rack): Companion override for the rack/cluster filter.
- [grafana_dashboard_var_service](/configuration/rack-parameters/aws/grafana_dashboard_var_service): Companion override for the service filter.
- [grafana_dashboard_var_app](/configuration/rack-parameters/aws/grafana_dashboard_var_app): Companion override for the app filter.
- [grafana_url](/configuration/rack-parameters/aws/grafana_url): Base Grafana URL used by the deep-link button.

## Version Requirements
This parameter requires at least Convox rack version `3.24.6`.
