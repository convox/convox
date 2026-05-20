---
title: "Budget Management"
slug: budget-management
url: /console/budget-management
---
# Budget Management

The Console provides per-App budget configuration and organization-wide cost visibility. Budget caps track month-to-date spend against a configurable threshold and enforce actions when the cap is reached.

## Prerequisites

- Rack version **3.24.6** or later
- The `cost_tracking_enable` Rack parameter set to `true` (enable via Rack Settings or `convox rack params set cost_tracking_enable=true`)

Without cost tracking enabled, budgets can be configured but will not accumulate spend or trigger actions.

## Cost Overview (Organization)

Navigate to **Organization > Cost Overview** to see aggregate spend across all Apps and Racks.

The overview displays:

- **Total month-to-date spend** across all tracked Apps
- **Per-App table** sortable by App name, Rack, Service count, MTD spend, and last updated time
- **Date range picker** for custom time windows (defaults to current month)
- **CSV export** for the displayed data

Click any row to navigate to that App's Budget tab.

Informational banners surface when:

- One or more Racks are unresponsive (stale data)
- Apps run on pre-3.24.6 Racks (cost not tracked)
- Racks have `cost_tracking_enable` set to `false` (Apps show $0)
- Apps have Services on unpriced instance types (displayed spend under-counts actual cloud bill)

## Per-App Budget Configuration

Navigate to **Organization > Rack > App > Budget** to configure an individual App's budget.

### Budget Header

The header card shows:

- **Month-to-date spend** against the configured cap
- **Progress bar** color-coded green (under 80%), yellow (80-100%), or red (over cap)
- **Last updated** timestamp

### Configuration Form

| Field | Description | Range |
|---|---|---|
| Monthly Budget Cap | Hard cap on monthly spend in USD | $0.01 to $100,000 |
| Alert Threshold | Percentage of cap at which alert notifications fire | 1% to 100% (default: 80%) |
| At-Cap Action | Enforcement behavior when the cap is reached | See below |
| Pricing Adjustment | Multiplier applied to recorded spend to match actual cloud invoices | 0.1 to 1.5 (default: 1.0) |

The pricing adjustment accounts for Enterprise Discount Programs, Savings Plans, or Reserved Instance commitments. Set below 1.0 to reduce reported costs (e.g., 0.85 for a 15% discount). Spot pricing is accounted for automatically.

### At-Cap Actions

| Action | Behavior |
|---|---|
| Alert Only | Send notifications (Slack, Discord) when cap is reached. No enforcement. |
| Block New Deploys | Reject `release promote` with a 409 error until the budget resets or rolls over to the next month. |
| Auto-Shutdown | Scale all eligible Services to 0 replicas after a 30-minute grace period. Services listed in the `budget.neverAutoShutdown` array in convox.yml are excluded. See [Budget Caps](/management/budget-caps) for shutdown ordering and eligibility. |

All changes are audit-logged with the acting user's email.

### Saving and Clearing

- **Save** persists the budget configuration. Changes apply immediately.
- **Clear** removes the budget entirely, including all enforcement rules.

Both actions take effect immediately and revert automatically if the save fails.

## Auto-Shutdown Lifecycle

When the at-cap action is set to Auto-Shutdown, the system follows a state machine:

### Armed

Budget cap reached. A banner displays a countdown timer (default 30 minutes). During this window:

- **Raise Cap:** Opens a dialog to increase the monthly cap above current spend, which cancels the shutdown.
- **Cancel Shutdown:** Resets the budget state without changing the cap.

### Active

Grace period expired. All eligible Services have been scaled to 0 replicas. The banner shows how many Services were affected and when shutdown occurred.

- **Restore Now:** Immediately restores all Services to their original replica counts and applies a 24-hour cooldown before auto-shutdown can re-arm.

### Recovered

Services have been restored. A success banner confirms recovery and displays any cooldown period.

- **Dismiss Banner:** Acknowledges the recovery and clears the banner.

### Failed

Shutdown or restore operation failed. The banner shows the failure reason (Kubernetes API failure, state corruption, admission webhook rejection, etc.).

- **Reset Budget Cap:** Clears the failed state and returns the App to normal.
- **Investigate:** Opens the budget caps documentation.

### Cap Raise Dialog

Available during the Armed state or from the Budget configuration:

- Displays current cap and current spend with percentage
- New cap must exceed both current cap and current spend
- Pre-fills with a suggested value (current cap x 1.5, rounded to nearest $50)
- If auto-shutdown is armed, raising above current spend cancels the scheduled shutdown

### Budget Reset

Resets the budget enforcement state:

- Re-enables normal operations and new deploys
- For Active (shutdown) state: restores Services to original replica counts with 24-hour cooldown
- For Armed state: cancels the scheduled shutdown
- Force-clear cooldown available via CLI only: `convox budget reset --force-clear-cooldown <app>` (Administrator role required)

## Per-App Cost Breakdown

Below the budget configuration, the Cost Breakdown section displays per-Service spend with:

- **Service-level rows** showing instance type, capacity type (on-demand vs. spot), and accumulated cost
- **Date range filtering** and Service name filtering
- **Aggregate toggle** to group by Service or show individual breakdowns
- **Warning banner** when pods run on unpriced instance types
- **Reset Period** (Administrators only) to zero accumulated spend and restart the billing period

## See Also

- [Budget Caps (CLI Reference)](/management/budget-caps)
- [Cost Tracking](/management/cost-tracking)
- [GPU Dashboard](/console/gpu-dashboard)
- [Model Deploy Wizard](/console/deploy-wizard)
- [Service Detail](/console/service-detail)
