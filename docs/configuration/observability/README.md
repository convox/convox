---
title: "Observability"
slug: observability
url: /configuration/observability
---
# Observability

Convox provides built-in logging and monitoring capabilities to help you understand what your applications are doing in production.

## Logging

Application logs are automatically captured from stdout and stderr of all running processes. State changes, health check results, and system events are also logged. Use `convox logs` to stream logs from the CLI.

See [Logging](/configuration/logging) for details.

## Monitoring and Alerting

Convox includes metrics collection, dashboards, and alerting capabilities. You can configure custom panels, PromQL queries, and notification channels (Slack, Discord).

See [Monitoring and Alerting](/configuration/monitoring) for details.

## Integrations

For advanced observability, Convox integrates with third-party monitoring platforms:

- [Datadog](/integrations/monitoring/datadog) for metrics, APM, and log management
