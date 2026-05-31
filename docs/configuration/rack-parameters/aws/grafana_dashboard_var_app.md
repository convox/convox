---
title: "grafana_dashboard_var_app"
slug: grafana_dashboard_var_app
url: /configuration/rack-parameters/aws/grafana_dashboard_var_app
---

# grafana_dashboard_var_app

## Description
The `grafana_dashboard_var_app` parameter overrides the Grafana dashboard template variable name for the app filter. The Console's "Open in Grafana" deep-link button substitutes this name into the URL it opens (e.g., `?var-app=<app>` becomes `?var-application=<app>` when set to `application`).

## Default Value
The default value is `app`.

## Allowed Range
Letters, digits, and underscore only. Values containing any other character are rejected, since they would break URL substitution or Grafana template syntax.

## Use Cases
- **Dashboards using `application`**: More verbose dashboard naming conventions favor `application` over `app`.
- **Dashboards using `project`**: Multi-tenant dashboards sometimes scope by `project` instead of `app`.

## Setting Parameters
To override to `application`:
```bash
$ convox rack params set grafana_dashboard_var_app=application -r rackName
Setting parameters... OK
```

To revert to the default:
```bash
$ convox rack params set grafana_dashboard_var_app=app -r rackName
Setting parameters... OK
```

To clear the override (falls back to the canonical default `app`):
```bash
$ convox rack params set grafana_dashboard_var_app= -r rackName
Setting parameters... OK
```

## Operational Notes
- No restart is required. The next "Open in Grafana" deep-link uses the new value.
- The "Dashboard filter mismatch?" troubleshooting modal in the Console explains the four configurable var names and how to inspect Grafana's expected variable names.

## Related Parameters
- [grafana_dashboard_var_rack](/configuration/rack-parameters/aws/grafana_dashboard_var_rack): Companion override for the rack/cluster filter.
- [grafana_dashboard_var_namespace](/configuration/rack-parameters/aws/grafana_dashboard_var_namespace): Companion override for the namespace filter.
- [grafana_dashboard_var_service](/configuration/rack-parameters/aws/grafana_dashboard_var_service): Companion override for the service filter.
- [grafana_url](/configuration/rack-parameters/aws/grafana_url): Base Grafana URL used by the deep-link button.

## Version Requirements
This parameter requires at least Convox rack version `3.24.6`.
