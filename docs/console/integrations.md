---
title: "Integrations"
description: "Connect the Console to your cloud providers, source control repositories, and notification channels through the Runtimes, Source, and Notification tabs."
slug: integrations
url: /console/integrations
---
# Integrations

Integrations connect the Console to your cloud providers, source control repositories, and notification channels. Navigate to **Infrastructure > Integrations** in the Console sidebar.

The Integrations page has three tabs: **Runtimes**, **Source**, and **Notification**.

## Runtime Integrations

Runtime integrations allow the Console to create and manage Racks on your cloud infrastructure.

### Supported Providers

| Provider | Setup Method |
|----------|-------------|
| **Amazon Web Services** | CloudFormation stack creates an IAM role for Console |
| **AWS GovCloud** | CloudFormation stack (GovCloud partition) |
| **Google Cloud** | Service account credentials |
| **Microsoft Azure** | Service principal credentials (Subscription ID, Tenant ID, Client ID, Client Secret) |
| **DigitalOcean** | API token |

### Adding a Runtime Integration

1. Select the **Runtimes** tab
2. Click **Create Runtime**
3. Select your cloud provider
4. Complete the provider-specific setup:
   - **AWS / AWS GovCloud:** Click **Launch Stack** to create the IAM role via CloudFormation in your AWS account. No billing fees for the role itself.
   - **Azure:** Enter your Subscription ID, Tenant ID, Client ID, and Client Secret.
   - **GCP:** Provide service account credentials with the required permissions.
   - **DigitalOcean:** Enter your API token.

Once connected, the Console can install and manage Racks on that provider account.

## Source Integrations

Source integrations connect your code repositories to the Console, enabling [Workflows](/console/workflows) for automated building, testing, and deployment.

### Supported Providers

| Provider | Setup |
|----------|-------|
| **GitHub** | OAuth authorization |
| **GitLab** | OAuth authorization |

### Adding a Source Integration

1. Select the **Source** tab
2. Click the **+** button and select **GitHub** or **GitLab**
3. Authorize Console access to your repositories through the OAuth flow
4. Your repositories become available when creating Workflows

After connecting, the Console receives webhook events when code is pushed or pull requests are opened. These events trigger any matching [Deployment Workflows](/console/workflows#deployment-workflows) or [Review Workflows](/console/workflows#review-workflows).

## Notification Integrations

Notification integrations forward Rack and Workflow events to your team. See [Notifications](/console/notifications) for the full list of events and configuration details.

### Supported Channels

| Channel | Setup |
|---------|-------|
| **Slack** | OAuth authorization |
| **Discord** | OAuth authorization |

### Adding a Notification Integration

1. Select the **Notification** tab
2. Click the **+** button and select **Slack** or **Discord**
3. Complete the OAuth authorization flow

## Managing Integrations

Each integration appears in its tab with its name and provider icon. Click the settings control on an integration to view details or remove it.

Removing a runtime integration does not uninstall Racks that were created through it. Removing a source integration disables all Workflows that reference its repositories. Removing a notification integration stops event forwarding immediately.

## See Also

- [Workflows](/console/workflows)
- [Notifications](/console/notifications)
- [Console Overview](/console/overview)
