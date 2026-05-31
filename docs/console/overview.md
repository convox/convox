---
title: "Console"
description: "The Convox Console is the web management interface for managing Racks, Apps, deployments, team access, and integrations across all your cloud providers."
slug: overview
url: /console/overview
---
# Console

The Convox Console is the web management interface for the Convox platform. It provides a centralized dashboard to manage Racks, Apps, deployments, team access, and integrations across all your cloud providers.

Access the Console at [console.convox.com](https://console.convox.com).

## Sidebar Navigation

The Console sidebar organizes all platform functionality into five sections.

### Metrics

| Page | Description |
|------|-------------|
| **Dashboard** | Overview of Rack and App health across your organization. Shows resource utilization, recent deployments, and active alerts at a glance. |
| **Alert Manager** | Configure alert rules for Rack and App events. Set thresholds, choose notification channels, and manage alert lifecycle (firing, acknowledged, resolved). |

### Infrastructure

| Page | Description |
|------|-------------|
| **Racks** | Install, update, and manage Racks across AWS, GCP, Azure, and DigitalOcean. Select a Rack to access its Apps, Instances, Processes, Resources, Logs, and Settings. |
| **Cost Overview** | Organization-wide cost tracking. View month-to-date spend across all Apps and Racks, filter by date range, and export to CSV. See [Cost Tracking](/management/cost-tracking). |
| **Integrations** | Connect cloud providers (runtime), code repositories (source), and notification channels (Slack, Discord). See [Integrations](/console/integrations). |

### CI/CD

| Page | Description |
|------|-------------|
| **Workflows** | Automated Build and Deploy pipelines triggered by repository events. Supports Deployment Workflows (push-to-deploy) and Review Workflows (PR preview environments). See [Workflows](/console/workflows). |
| **Jobs** | History and status of every Workflow-triggered Build and deployment. Filter by status, App, or Workflow. |
| **Deploy Keys** | API keys for CLI-based and headless deployments. Used by CI systems and automation scripts that deploy without a user session. |

### Auditing

| Page | Description |
|------|-------------|
| **Audit Logs** | Searchable history of every action taken across your organization. Each entry includes the timestamp, action type, target resource, and the user who performed it. |

### Settings

| Page | Description |
|------|-------------|
| **Users** | Invite and manage team members, assign organization roles, and control access to Racks and Apps. See [RBAC](/management/rbac). |
| **Billing** | Manage your subscription plan, payment method, and invoice history. |
| **Settings** | Organization-level configuration including organization name and SAML SSO setup. |

## App Pages

Select a Rack from the sidebar, then select an App to access the following pages:

| Page | Description |
|------|-------------|
| **Services** | Running Services with their endpoints, replica counts, and resource allocation. Click a Service name to open its [detail view](/console/service-detail). |
| **Builds** | Build history with logs. Each Build shows the source commit, duration, status, and manifest used. |
| **Processes** | Running Process list showing status, uptime, CPU and memory usage per replica. |
| **Releases** | Release history with promote and rollback controls. Each Release links to its Build and shows the deployment status. |
| **Environment** | View and edit environment variables for the App. Sensitive values are masked by default. |
| **Events** | Timeline of lifecycle events including deploys, promotes, rollbacks, scale changes, and budget events. |
| **GPU Telemetry** | Real-time GPU utilization, memory, tensor core activity, and power draw for accelerated Services. See [GPU Dashboard](/console/gpu-dashboard). |
| **Budget** | Per-App spend tracking with configurable caps, alert thresholds, and auto-shutdown enforcement. See [Budget Management](/console/budget-management). |

## Getting Started

1. Sign up or log in at [console.convox.com](https://console.convox.com)
2. Create an organization
3. Add a [runtime integration](/console/integrations) to connect your cloud provider
4. Install a Rack on your cloud account
5. Add a [source integration](/console/integrations) to connect GitHub or GitLab
6. Create a [Workflow](/console/workflows) to automate deployments

## See Also

- [Workflows](/console/workflows)
- [Integrations](/console/integrations)
- [Notifications](/console/notifications)
- [Rack Roles](/console/rack-roles)
- [Webhook Signing](/console/webhook-signing)
- [GPU Dashboard](/console/gpu-dashboard)
- [Budget Management](/console/budget-management)
- [Model Deploy Wizard](/console/deploy-wizard)
- [Service Detail](/console/service-detail)
- [Autoscale Triggers](/console/autoscale-triggers)
- [Rack Settings](/console/rack-settings)
