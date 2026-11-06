---
title: "Notifications"
description: "Forward Rack and Workflow events to Slack or Discord, covering App lifecycle, build and release, infrastructure, workflow, alert, and budget events."
slug: notifications
url: /console/notifications
---
# Notifications

Notification integrations forward events from your Racks and Workflows to your team's communication channels.

## Supported Channels

| Channel | Setup |
|---------|-------|
| **Slack** | OAuth connection. Click the Slack provider in the Console and authorize access to your workspace. |
| **Discord** | OAuth connection. Click the Discord provider and authorize the webhook. |

## Configuration

1. Navigate to **Infrastructure > Integrations** in the Console sidebar
2. Select the **Notification** tab
3. Click the **+** button and select **Slack** or **Discord**
4. Complete the OAuth authorization flow
5. The integration appears in the Notification list

Each notification integration forwards all events from all Racks in the organization. Remove the integration to stop notifications.

## Events

The following events are forwarded to notification channels:

### App Lifecycle

| Event | Description |
|-------|-------------|
| `app:create` | App created |
| `app:delete` | App deleted |

### Build and Release

| Event | Description |
|-------|-------------|
| `build:create` | Build started |
| `release:create` | Release created |
| `release:promote` | Release promoted, started, or failed |
| `release:finish` | Rolling deploy of a Release finished |
| `release:scale` | Service scaled |
| `app:promote:completed` | Release deployment completed successfully |
| `app:promote:errored` | Release deployment failed |
| `app:promote:cancelled` | Release deployment superseded by a newer Release |

### Infrastructure

| Event | Description |
|-------|-------------|
| `resource:create` | Resource created |
| `resource:delete` | Resource deleted |
| `system:update` | Rack updated (version, instance count, or instance type) |

### Workflow

| Event | Description |
|-------|-------------|
| `workflow:complete` | Deployment or Review workflow completed, cancelled, or failed |

### Alert

| Event | Description |
|-------|-------------|
| `alert:firing` | Alert is firing |
| `alert:resolved` | Alert has resolved |

### Budget

| Event | Description |
|-------|-------------|
| `app:budget:threshold` | Spend threshold alert triggered |
| `app:budget:cap` | Budget cap reached |
| `app:budget:auto-shutdown:armed` | Auto-shutdown scheduled |
| `app:budget:auto-shutdown:fired` | Services scaled to zero by auto-shutdown |
| `app:budget:auto-shutdown:cancelled` | Auto-shutdown cancelled |
| `app:budget:auto-shutdown:restored` | App restored from auto-shutdown |
| `app:budget:auto-shutdown:expired` | Auto-shutdown expired, manual recovery required |
| `app:budget:auto-shutdown:flap-suppressed` | Auto-shutdown suppressed (re-trip within cooldown) |
| `app:budget:auto-shutdown:failed` | Auto-shutdown or restore operation failed |
| `app:budget:auto-shutdown:simulated` | Auto-shutdown simulation result |

Events that modify budget configuration (`app:budget:set`, `app:budget:clear`, `app:budget:reset`) are recorded in the audit log but are not forwarded to notification channels. Override events (`app:triggers-override:toggled`, `app:triggers-override:threshold-set`, `app:scale-override:toggled`, `app:scale-override:honored`) are also audit-only.

## Message Format

Notifications include the Rack name, event type, status, and relevant details (App name, Release ID, actor). Slack messages use Slack formatting; Discord messages use Discord-native formatting.

## See Also

- [Integrations](/console/integrations)
- [Workflows](/console/workflows)
- [Webhook Signing](/console/webhook-signing)
