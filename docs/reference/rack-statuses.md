---
title: "Rack Statuses"
slug: rack-statuses
url: /reference/rack-statuses
---
# Rack Statuses

Every Rack has a status that reflects its infrastructure lifecycle state. The status is visible on the Console Racks page, in the Rack detail header, and via the CLI with `convox rack`.

## Status Reference

| Status | Description | Available Actions |
|--------|-------------|-------------------|
| `installing` | The Rack is being installed. Terraform is provisioning cloud infrastructure (VPC, EKS/GKE/AKS cluster, load balancers, etc.). | Wait for installation to complete. The Console card is non-interactive during this state. |
| `incomplete` | Installation started but did not finish. The Rack is partially provisioned. | Retry the installation or uninstall to clean up partial resources. The Console opens the uninstall dialog on click. |
| `running` | The Rack is healthy and operational. All infrastructure is provisioned and the Rack API is responsive. | Full access: deploy Apps, manage parameters, update the Rack version, view metrics, configure settings. |
| `updating` | A parameter change or version update is in progress. Terraform is applying infrastructure changes. | View the Rack, Apps, and settings. Wait for the update to complete. Do not submit additional parameter changes until the update finishes (state lock will reject them). |
| `converging` | The Rack is reconciling infrastructure state after an update. Kubernetes resources are being rolled out or node groups are cycling. This status is set by the Console only and does not appear in `convox rack` CLI output. | View the Rack and Apps. Wait for convergence to complete. The Console polls at 5-second intervals during this state. |
| `rollback` | A Rack update failed and Terraform is reverting to the previous state. | View the Rack and Apps. Wait for the rollback to complete. Investigate the failure cause in Rack events or update history. |
| `deleting` | Resources within the Rack are being cleaned up (e.g., App deletion cascading through infrastructure). | View the Rack. Wait for deletion to complete. |
| `uninstalling` | The Rack is being uninstalled. Terraform is destroying all cloud infrastructure. | Wait for the uninstall to complete. The Console opens the uninstall progress dialog on click. |
| `failed` | The Rack is in a failed state. Installation or uninstallation did not complete. | Retry the uninstall to clean up remaining resources. The Console opens the uninstall dialog on click. |
| `unknown` | The Rack API is not responding. The Console cannot reach the Rack to determine its status. | The Console shows cached data with a stale-data banner. Navigate into the Rack to access cached App lists, settings, update history, and the Uninstall action. |

## Status Transitions

```text
installing ──► running ──► updating ──► running
    │              │            │
    │              │            └──► converging ──► running
    │              │            │
    │              │            └──► rollback ──► running
    │              │                    │
    │              │                    └──► failed
    │              │
    │              └──► uninstalling ──► (removed)
    │
    └──► incomplete ──► uninstalling ──► (removed)
```

## Console Behavior by Status

The Console adjusts its behavior based on Rack status:

- **Polling frequency**: 5 seconds for `installing`, `updating`, `uninstalling`, `converging`, `rollback`, and `deleting`. 30 seconds for `running`.
- **Card interactivity**:
  - `running` and `unknown` — click navigates into the Rack detail view
  - `updating`, `converging`, `rollback`, `deleting` — click navigates into the Rack detail view
  - `uninstalling`, `incomplete`, `failed` — click opens the uninstall dialog
  - `installing` — card is non-interactive
- **Stale-data handling**: when a Rack probe fails, the Console marks the status as `unknown` and displays a banner on all sub-pages indicating that data may be out of date.

## CLI Commands

Check the current Rack status:

```bash
$ convox rack
```

View Rack parameters (also confirms whether an update is still in progress):

```bash
$ convox rack params
```

If `convox rack params` returns a state-lock error, the Rack is still updating. Wait and retry.

## Waiting for Updates

Rack parameter changes and version updates are asynchronous. The `convox rack params set` command returns immediately, but the infrastructure change has not yet started. Check `convox rack params` periodically:

- A successful parameter listing means the update is complete.
- A state-lock error means the update is still in progress.

Typical update times:
- Simple parameter changes: 2-8 minutes
- Node type or architecture changes: 10-30 minutes
- KEDA, Karpenter, or GPU feature toggles: 10-30 minutes

## See Also

- [App Statuses](/reference/app-statuses) for the App lifecycle
- [Console Rack Management](/management/console-rack-management) for moving Racks between CLI and Console
- [CLI Rack Management](/management/cli-rack-management) for managing Racks from the command line
