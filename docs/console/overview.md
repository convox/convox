---
title: "Console"
slug: overview
url: /console/overview
---
# Console

The Convox Console is the web management interface for the Convox platform. It provides a centralized dashboard to manage Racks, Apps, deployments, team access, and integrations across all your cloud providers.

Access the Console at [console.convox.com](https://console.convox.com).

## Main Sections

The Console sidebar organizes functionality into the following sections:

### Metrics

- **Dashboard** -- overview of Rack and App health, resource utilization, and deployment activity
- **Alert Manager** -- configure and manage alerts for your infrastructure

### Infrastructure

- **Racks** -- install, update, and manage Racks across AWS, GCP, Azure, and DigitalOcean. Drill into individual Racks to view Apps, Instances, Processes, Resources, Logs, and Settings.
- **Cost Overview** -- track infrastructure costs across your organization
- **Integrations** -- connect cloud providers, source control, and notification channels. See [Integrations](/console/integrations) for details.

### CI/CD

- **Workflows** -- configure automated deployment and review pipelines triggered by repository events. See [Workflows](/console/workflows) for details.
- **Jobs** -- view the history and status of Workflow-triggered Builds and deployments
- **Deploy Keys** -- manage deploy keys for CLI-based and headless deployments

### Auditing

- **Audit Logs** -- searchable history of every action taken across your organization, tagged with the actor who performed it

### Settings

- **Users** -- invite and manage team members, assign roles, and control access
- **Billing** -- manage your subscription plan and payment details
- **Settings** -- organization-level configuration (name, SAML SSO, etc.)

## App Management

Select a Rack, then select an App to access:

- **Services** -- view running Services and their endpoints
- **Builds** -- Build history and logs
- **Processes** -- running Process list with status and resource usage
- **Releases** -- Release history with promote and rollback controls
- **Environment** -- manage environment variables
- **Events** -- lifecycle event timeline (deploys, promotes, rollbacks, budget events)
- **GPU Dashboard** -- real-time GPU telemetry for accelerated Services. See [GPU Dashboard](/console/gpu-dashboard).
- **Budget** -- per-App spend tracking, caps, and enforcement. See [Budget Management](/console/budget-management).
- **Service Detail** -- drill into individual Service health, scaling, and resource usage. See [Service Detail](/console/service-detail).

## Getting Started

1. Sign up or log in at [console.convox.com](https://console.convox.com)
2. Create an organization
3. Add a [runtime integration](/console/integrations) to connect your cloud provider
4. Install a Rack on your cloud account
5. Add a [source integration](/console/integrations) to connect GitHub or GitLab
6. Create a [Workflow](/console/workflows) to automate deployments

## See Also

- [Workflows](/console/workflows) -- automated deployment and review pipelines
- [Integrations](/console/integrations) -- runtime, source, and notification connections
- [Notifications](/console/notifications) -- Slack and Discord event forwarding
- [Rack Roles](/console/rack-roles) -- organization administrator gate for sensitive operations
- [Webhook Signing](/console/webhook-signing) -- verify webhook payload authenticity
- [GPU Dashboard](/console/gpu-dashboard) -- real-time GPU telemetry
- [Budget Management](/console/budget-management) -- per-App spend tracking and caps
- [Deploy Wizard](/console/deploy-wizard) -- guided deployment setup
- [Service Detail](/console/service-detail) -- per-Service health and scaling
- [Autoscale Triggers](/console/autoscale-triggers) -- Console-driven autoscaling
- [Rack Settings](/console/rack-settings) -- Rack-level configuration
