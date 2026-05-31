---
title: "App Statuses"
description: "App statuses report each App's lifecycle state (creating, running, updating, rollback, deleting, failed) and the actions allowed in each state."
slug: app-statuses
url: /reference/app-statuses
---
# App Statuses

Every App in Convox has a status that reflects its current lifecycle state. The status is visible in the Console App list, the App detail header, and via the CLI with `convox apps` or `convox apps info`.

## Status Reference

| Status | Description | Available Actions |
|--------|-------------|-------------------|
| `creating` | The App is being created. Infrastructure provisioning is in progress. | Wait for creation to complete. No deploy or scale actions are available. |
| `running` | The App is healthy and serving traffic. All Services are at their target replica counts. | Deploy, scale, restart, promote Releases, update environment, delete. Full Console and CLI access. |
| `updating` | A deploy, environment change, or scale operation is in progress. The App transitions to this status when a new Release is promoted or environment variables are changed. | View logs and events. Wait for the update to complete. The Console polls at a faster interval (5s) during this state. |
| `rollback` | The App is rolling back to a previous Release after a failed deploy. Convox automatically triggers a rollback when health checks fail during a promotion. | View logs and events to diagnose the failure. Wait for the rollback to complete. The Console polls at a faster interval (5s) during this state. |
| `deleting` | The App is being deleted. Kubernetes resources and associated infrastructure are being torn down. | Wait for deletion to complete. No other actions are available. |
| `failed` | The App is in a failed state. A deploy, rollback, or infrastructure operation did not complete. | Investigate via logs and events. Promote a known-good Release to recover, or delete the App. |

## Status Transitions

```text
creating ──► running ──► updating ──► running
                │            │
                │            └──► rollback ──► running
                │                    │
                │                    └──► failed
                │
                └──► deleting
```

- **creating → running**: App infrastructure is provisioned and the initial Release (if any) is promoted.
- **running → updating**: A new Release promotion, environment change, or scale operation begins.
- **updating → running**: The update completes and all Services reach their target state.
- **updating → rollback**: Health checks fail during the update. Convox reverts to the previous Release.
- **rollback → running**: The rollback completes and Services return to the previous known-good state.
- **rollback → failed**: The rollback itself fails. Manual intervention is needed.
- **running → deleting**: `convox apps delete` or Console delete action is invoked.

## Console Behavior by Status

The Console adjusts its behavior based on App status:

- **Polling frequency** increases to 5 seconds during `updating` and `rollback` (default is 15 seconds for `running`).
- **Deploy and promote buttons** are disabled while the App is in `creating`, `updating`, `rollback`, or `deleting`.
- **Status badge color** follows this mapping:
  - `running`: green (success)
  - `updating`: blue (info)
  - `rollback`: orange (warning)
  - `failed`: red (danger)
  - Other statuses: gray (secondary)

## CLI Commands

Check the current App status:

```bash
$ convox apps info -a <app-name>
```

List all Apps with their statuses:

```bash
$ convox apps
```

## See Also

- [Rack Statuses](/reference/rack-statuses) for the Rack lifecycle
- [Releases](/reference/releases) for understanding the deploy and promote cycle
- [Scaling](/configuration/scaling) for Service scaling configuration
