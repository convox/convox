---
title: "grafana_dashboard_var_service"
description: "The grafana_dashboard_var_service AWS rack parameter overrides the Grafana dashboard template variable name for the service filter, defaulting to service."
slug: grafana_dashboard_var_service
url: /configuration/rack-parameters/aws/grafana_dashboard_var_service
---

# grafana_dashboard_var_service

## Description
The `grafana_dashboard_var_service` parameter overrides the Grafana dashboard template variable name for the service filter. The Console's "Open in Grafana" deep-link button substitutes this name into the URL it opens (e.g., `?var-service=<svc>` becomes `?var-workload=<svc>` when set to `workload`).

## Default Value
The default value is `service`.

## Allowed Range
Letters, digits, and underscore only. Values containing any other character are rejected, since they would break URL substitution or Grafana template syntax.

## Use Cases
- **Dashboards using `workload`**: Some Kubernetes-native dashboards use the broader `workload` term to encompass Deployments, StatefulSets, and DaemonSets uniformly.
- **Dashboards using `deployment`**: Convox `service` resources map to K8s Deployments; some dashboards label the variable accordingly.

## Setting Parameters
To override to `workload`:
```bash
$ convox rack params set grafana_dashboard_var_service=workload -r rackName
Setting parameters... OK
```

To revert to the default:
```bash
$ convox rack params set grafana_dashboard_var_service=service -r rackName
Setting parameters... OK
```

To clear the override (falls back to the canonical default `service`):
```bash
$ convox rack params set grafana_dashboard_var_service= -r rackName
Setting parameters... OK
```

## Operational Notes
- No restart is required. The next "Open in Grafana" deep-link uses the new value.
- The "Dashboard filter mismatch?" troubleshooting modal in the Console explains the four configurable var names and how to inspect Grafana's expected variable names.

## Related Parameters
- [grafana_dashboard_var_rack](/configuration/rack-parameters/aws/grafana_dashboard_var_rack): Companion override for the rack/cluster filter.
- [grafana_dashboard_var_namespace](/configuration/rack-parameters/aws/grafana_dashboard_var_namespace): Companion override for the namespace filter.
- [grafana_dashboard_var_app](/configuration/rack-parameters/aws/grafana_dashboard_var_app): Companion override for the app filter.
- [grafana_url](/configuration/rack-parameters/aws/grafana_url): Base Grafana URL used by the deep-link button.

## Version Requirements
This parameter requires at least Convox rack version `3.24.6`.
