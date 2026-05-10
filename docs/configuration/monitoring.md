---
title: "Monitoring and Alerting"
slug: monitoring-and-alerting
url: /configuration/monitoring
---
# Monitoring and Alerting

Convox provides integrated metrics collection and alerting capabilities that give you comprehensive visibility into your applications and infrastructure. Built directly into the Convox platform, monitoring requires no additional tools or complex configuration.

## Monitoring Capabilities

Convox Monitoring and Alerting offers:

- **Automatic metrics collection** from your racks and applications
- **Pre-configured dashboards** with essential infrastructure metrics
- **Custom panel creation** using Smart Queries or PromQL
- **Intelligent alerting** with configurable thresholds and notifications
- **Seamless integration** with your existing Convox workflow

## Enabling Metrics Collection

Metrics collection is enabled at the rack level through the Rack Settings page.

### Steps to Enable

1. Navigate to your rack in the Convox Console
2. Click **Rack Settings** in the left sidebar
3. Scroll to the **Dashboard Settings** section
4. Toggle the **Enable Metrics Agent** switch to on
5. In the plan picker modal, choose **Free** (lightweight Prometheus chart, no metered billing) or **Paid** (kube-prometheus-stack with metered billing; requires payment method on file).
6. Wait approximately 5-15 minutes for the chart to install. Status updates in the same panel.

Once enabled, the Metrics Agent installs into your rack and begins collecting performance data from all running services.

### Switching Plans

When monitoring is enabled, the Rack Settings → Dashboard Settings panel shows a **Switch Plan** button alongside the existing toggle. Switching between Free and Paid:

- Uninstalls the current plan's chart, then installs the new plan's chart. Total ~5-15 minutes.
- During the switch, `convox ps` GPU enrichment fields show em-dash sentinels until the new chart is installed and `prometheus_url` is wired (see below).
- Free → Paid requires a payment method on file. The Console surfaces a card-on-file gate before the switch begins.
- Paid → Free preserves the underlying Stripe subscription (metering stops; no auto-cancellation).

### Setting `prometheus_url` for `convox ps` GPU enrichment

Post-3.24.6, the rack does not auto-resolve a Prometheus URL. To populate GPU fields in `convox ps` after enabling monitoring, set [`prometheus_url`](/configuration/rack-parameters/aws/prometheus_url) on your rack.

> **Note:** the free-plan Prometheus chart depends on the Convox Console monitoring-redesign deploy. If your Convox Console version does not yet show the **Free** vs **Paid** plan picker in Rack Settings → Dashboard Settings, the free-plan service URL below will not resolve (no chart deployed). Use the paid-plan URL or wait until the redesign rolls out to your Convox Console.



```bash
# Paid plan in-cluster Prometheus:
convox rack params set prometheus_url=http://convox-kube-prometheus-sta-prometheus.convox-monitoring.svc.cluster.local:9090

# Free plan in-cluster Prometheus:
convox rack params set prometheus_url=http://prometheus-gpu-metrics-server.kube-system.svc.cluster.local:80
```

Until `prometheus_url` is set, `convox ps` GPU fields render as em-dash sentinels (`—`) even after monitoring is enabled in the Console.

## Default Dashboards

When you enable metrics, Convox automatically creates default panels that provide immediate visibility into your infrastructure:

- **Rack CPU Usage** - Overall CPU utilization across your rack
- **Rack Memory Usage** - Memory consumption patterns
- **Rack Network I/O** - Network throughput and activity
- **Rack Network Errors** - Network error rates
- **Application Pod Count** - Number of running application instances
- **Application Pod Ready %** - Percentage of healthy application pods

These panels give you a comprehensive baseline view of your infrastructure health without any additional configuration.

## Custom Panels

Create custom panels to track specific metrics that matter to your applications.

### Creating a Panel

1. Navigate to the **V3 Dashboard** in the Metrics section
2. Click **Create Panel**
3. Configure your panel:
   - **Name**: Descriptive name for your panel
   - **Description**: Optional details about what the panel tracks
   - **Chart Type**: Visualization type (automatically set for Smart Queries)

### Adding Queries

Panels can contain multiple queries to compare different metrics or services.

#### Smart Queries

Smart Queries are pre-configured queries that make metric selection intuitive:

1. Select your **Rack** from the dropdown
2. Choose **All Apps** or select a specific application
3. Choose **All Services** or select specific services
4. Select a metric from the **Smart Query Metrics** list
5. Click **Generate Preview** to verify the query

Available Smart Query metrics include:
- CPU Usage Trend
- Memory Usage Trend
- Network I/O patterns
- Request rates and response times

#### Custom Queries

For advanced monitoring needs, you can input any valid PromQL query to access the full range of collected metrics.

### Panel Management

- **Generate Preview**: View your panel before saving
- **Create Panel**: Save the panel to your dashboard
- **Edit/Delete**: Modify existing panels using the panel controls

## Alerting

Set up intelligent alerts to be notified when your applications exceed defined thresholds.

### Creating Alert Rules

1. Navigate to **Alert Manager** in the Metrics section
2. Click the settings icon to create a new alert
3. Configure your alert:

#### Basic Configuration
- **Rule Name**: Descriptive name for the alert
- **Severity**: Choose from Warning, Critical, or other severity levels
- **Panel**: Select the panel containing the metric to monitor
- **Query**: Choose the specific query within the panel

#### Conditions
- **Condition**: Set when the alert should trigger (greater than, less than, etc.)
- **Threshold**: The value that triggers the alert
- **For Seconds**: How long the condition must persist before triggering
- **Keep Firing For Seconds**: How long to keep the alert active once triggered

#### Documentation
- **Summary**: Brief description used in alert notifications
- **Description**: Detailed explanation of what the alert monitors
- **Labels**: Key-value pairs for organizing and routing alerts

### Alert Management

The Alert Manager dashboard shows:
- **Active Alerts**: Currently firing alerts
- **Total Alerts**: All configured alert rules
- **Monitoring Status**: Overall system health
- **Alert History**: List of all alert rules with their current state

## Notifications

Configure notification integrations to receive alerts where your team works.

### Supported Integrations

- **Slack**: Send alerts directly to Slack channels
- **Discord**: Deliver notifications to Discord channels
- **PagerDuty**: Route alerts to PagerDuty for on-call workflows

### Setting Up Notifications

1. Navigate to the **Integrations** page
2. Select your preferred notification service
3. Follow the integration-specific setup process
4. Test the integration to ensure alerts are delivered properly

## Best Practices

### Panel Organization
- Create panels that group related metrics for easier analysis
- Use descriptive names and descriptions for panels and queries
- Combine multiple services in a single panel to compare performance

### Alert Configuration
- Set appropriate thresholds based on your application's normal operating ranges
- Use "For Seconds" to avoid alert noise from temporary spikes
- Include clear summaries and descriptions for faster incident response
- Apply consistent labeling for better alert organization

### Monitoring Strategy
- Start with default panels to understand baseline performance
- Create custom panels for application-specific metrics
- Set up alerts for critical thresholds that require immediate attention
- Review and adjust alert thresholds based on historical data

## Troubleshooting

### Metrics Not Appearing
- Verify the Metrics Agent is enabled in Rack Settings
- Wait 2-3 minutes for agents to fully initialize
- Check that your applications are running and generating metrics

### Alert Issues
- Ensure alert conditions and thresholds are correctly configured
- Verify notification integrations are properly set up
- Check alert timing settings (For Seconds, Keep Firing For Seconds)

### Performance Impact
Convox monitoring is designed to have minimal impact on your applications. The Metrics Agent collects data efficiently without affecting application performance.

## Command Line Interface

While monitoring is primarily managed through the Convox Console, you can view application logs using the CLI:

```bash
# View application logs
convox logs -a myapp

# Filter logs by service
convox logs -a myapp --service web

# View logs for a specific time period
convox logs -a myapp --since 1h

# Stream application logs (default behavior)
convox logs -a myapp
```

## User-Visible Regressions in 3.24.6

Acknowledged regressions introduced by the monitoring ownership redesign. Each is documented to set expectations:

- **Rack-only users (no Convox Console connection) lose access to GPU-observability Prometheus charts.** The DCGM exporter still installs via `gpu_observability_enable=true`, but no Prometheus chart scrapes it. Connect to Convox Console and enable monitoring to restore full functionality.
- **`convox rack params` no longer shows `monitoring_metrics_provisioned`.** Diagnostic visibility moves to the Console audit log + UI rack-detail panel.
- **`convox rack params set monitoring_metrics_provisioned=...` returns an error**: "removed in 3.24.6; monitoring is now Convox-Console-driven (Convox Console → Rack Settings → Dashboard Settings → Enable Metrics Agent)". One-time user surprise.
- **Chart-version overrides require Disable→Enable cycle to take effect.** Setting [`prometheus_gpu_metrics_chart_version`](/configuration/rack-parameters/aws/prometheus_gpu_metrics_chart_version) or [`prometheus_gpu_metrics_retention`](/configuration/rack-parameters/aws/prometheus_gpu_metrics_retention) on a rack with monitoring enabled does not immediately re-deploy — the new value applies on the next Disable→Enable cycle from the Console.
- **`convox ps` GPU fields show em-dash sentinels when `prometheus_url` is unset.** Even with monitoring enabled in the Console, the rack does not auto-resolve a Prometheus URL. See "Setting `prometheus_url`" above.
- **DCGM-only user state**: a user who sets `gpu_observability_enable=true` on the rack but never enables monitoring in the Console has DCGM running with no scraper. User dashboards stay empty until monitoring is enabled in the Console.
- **api-pod rolling restart at upgrade**: ~30-90s rolling restart triggered by the `prometheus_url` secret-checksum hash change. Expected behavior during the upgrade window.
- **Users using KEDA Prometheus-backed autoscaling (`scale.autoscale.gpuUtilization` or `scale.autoscale.queueDepth` without an explicit per-trigger `prometheusUrl`) MUST set `prometheus_url` explicitly.** The pre-3.24.6 implicit fallback is removed; only the Prometheus-backed trigger is skipped when `prometheus_url` is empty — CPU-, memory-, and `scale.keda.triggers`-based autoscale (e.g. `aws-sqs-queue`, `kafka`, `cron`) continue to render and work normally without a Prometheus URL. Set `prometheus_url` to restore Prometheus-backed autoscale.
- **Plan-switch requires 3.24.6+ rack version.** Users on downgraded (3.24.5 or earlier) racks cannot switch between free and paid plans via the Console; legacy disable still works for both plans. Upgrade rack to 3.24.6+ to use plan-switch.
- **Downgrade from 3.24.6 to 3.24.5 with monitoring enabled in Convox Console will fail at TF apply.** Console-installed `prometheus-gpu-metrics` chart in `kube-system` ns conflicts with 3.24.5's rack-TF-managed chart (helm "cannot re-use a name that is still in use"). **Mitigation:** disable monitoring in Convox Console BEFORE downgrading the rack (Rack Settings → Dashboard Settings → toggle off); wait ~5 minutes for chart uninstall to complete; then `convox rack update --version 3.24.5`.
- **Telemetry receivers no longer see the `monitoring_metrics_provisioned` field.** Removed from the rack telemetry payload alongside the parameter removal. Telemetry consumers parsing the field should treat it as removed (no schema migration required; the field simply stops appearing).
- **Free-chart memory budget**: the free-plan Prometheus chart includes the upstream chart's default scrape jobs (kubernetes-pods, kubernetes-service-endpoints, kubernetes-nodes) in addition to the DCGM scrape. Memory budget: <500Mi typical; <1024Mi peak; zero OOMKilled events at default 24h retention. Larger clusters may need to drop retention via [`prometheus_gpu_metrics_retention`](/configuration/rack-parameters/aws/prometheus_gpu_metrics_retention) to stay within the typical budget.
- **GraphQL `plan` arg defaults to `paid` server-side for backward compat** with old clients (cached browser bundles, third-party tooling). Mutations without a `plan` arg fall back to the prior `paid` behavior.
- **BillingEnabled-disabled environments (dev/staging/on-prem)**: when `settings.StripePublicKey=""`, paid-plan Enable creates no Stripe subscription. Plan is recorded as `paid` but `o.StripeMonitoringSubscription` stays empty. `sendBillingMeterEvent` skips. Dev-environment expectation; documented for on-prem operators.
- **Restart-mid-Enable user recovery time (~50min worst case)**: user clicks Enable; mutation returns "ok"; Console pod restarts before the chart install completes. State-machine guard rejects re-clicks on `enabling` status. Recovery: worker tick (≤5min) + helm install timeout (≤25min) + stale-pending recovery threshold (≤45min) + retry tick (5min). UI shows "Enabling..." spinner throughout. SRE manual override is the only fast-path.

## See Also

- [Logging](/configuration/logging) for configuring log collection
- [Datadog Integration](/integrations/monitoring) for detailed Datadog setup instructions
- [`prometheus_url`](/configuration/rack-parameters/aws/prometheus_url) for `convox ps` GPU enrichment migration
- [`gpu_observability_enable`](/configuration/rack-parameters/aws/gpu_observability_enable) for the DCGM exporter rack parameter
