---
title: "Monitoring & Alerting"
draft: false
slug: Monitoring & Alerting
url: /configuration/monitoring
---
# Monitoring & Alerting

Convox provides integrated metrics collection and alerting capabilities that give you comprehensive visibility into your applications and infrastructure. Built directly into the Convox platform, monitoring requires no additional tools or complex configuration.

## Overview

Convox Monitoring & Alerting offers:

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
5. Wait approximately 2 minutes for agents to install and begin collecting data

Once enabled, the Metrics Agent automatically installs into your rack and begins collecting performance data from all running services.

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
- **PagerDuty**: Coming soon

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

# Tail application logs
convox logs -a myapp --follow