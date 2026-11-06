---
title: "Budget Management"
description: "Configure per-App budget caps and organization-wide cost visibility in the Console, tracking month-to-date spend and enforcing actions when a cap is reached."
slug: budget-management
url: /console/budget-management
---
# Budget Management

The Console provides per-App budget configuration and organization-wide cost visibility. Budget caps track month-to-date spend against a configurable threshold and enforce actions when the cap is reached.

## Prerequisites

- A Rack on AWS at Rack version **3.24.6** or later, on Azure at **3.25.1** or later, or on GCP at **3.25.9** or later. The Console does not offer cost tracking or budgets on other providers.
- The `cost_tracking_enable` Rack parameter set to `true` (enable via Rack Settings or `convox rack params set cost_tracking_enable=true`)

While cost tracking is off, saving a budget fails: the Rack rejects a cap, alert threshold or at-cap action, and the Console shows its error, which points at `cost_tracking_enable`.

## Cost Overview (Organization)

Navigate to **Organization > Cost Overview** to see aggregate spend across all Apps and Racks.

The overview displays:

- **Total spend** across all tracked Apps, month to date or for the selected range
- **Per-App table** sortable by App name, Rack, Service count, MTD spend, and last updated time
- **Date range pickers** for a range of UTC days, defaulting to the current month (see [Date ranges](#date-ranges))
- **CSV export** for the displayed data

Click any row to navigate to that App's Budget tab.

### Date ranges

Date ranges read the daily cost history a Rack keeps from version `3.25.10`, for [`cost_tracking_history_days`](/configuration/rack-parameters/aws/cost_tracking_history_days) days, 62 by default. History starts when the Rack updates to `3.25.10`, and days are UTC.

- The Cost Overview pickers stay locked to month-to-date until at least one Rack the Console installed or updated is on `3.25.10` or later. An imported Rack does not unlock them, but once they are unlocked an imported Rack on `3.25.10` reports the range. While a range is set, Apps on older Racks are left out of the table, and a note gives their count.
- The pickers on an App's Cost Breakdown section are locked to month-to-date while the App's Rack is below `3.25.10`.
- With a range set, the summary, the spend column and the CSV header name the range.

Informational banners surface when:

- One or more Racks are unresponsive (stale data)
- Apps run on Racks that do not support cost tracking (cost not tracked)
- Racks have `cost_tracking_enable` set to `false` (spend is not updated)
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
| Block New Deploys | Reject `release promote`, scaling and `convox run` with a 409 error until the cap is raised above current spend or the month rolls over. A budget reset lifts the block only until the next accumulator tick if spend is still at or above the cap. |
| Auto-Shutdown | Scale all eligible Services to 0 replicas after a countdown set by `budget.notifyBeforeMinutes` in convox.yml, 30 minutes by default. Services listed in the `budget.neverAutoShutdown` array in convox.yml are excluded, and so are agent and stateful Services. See [Budget Caps](/management/budget-caps) for shutdown ordering and eligibility. |

All changes are audit-logged with the acting user's email.

### Saving and Clearing

- **Save** persists the budget configuration. Changes apply immediately.
- **Clear** removes the budget entirely, including all enforcement rules. It also deletes the App's stored spend, so month-to-date spend starts again from zero.

Both actions take effect immediately and revert automatically if the save fails.

## Auto-Shutdown Lifecycle

When the at-cap action is set to Auto-Shutdown, the system follows a state machine:

### Armed

Budget cap reached. A banner displays a countdown timer (default 30 minutes). During this window:

- **Raise Cap:** Opens a dialog to increase the monthly cap above current spend, which cancels the shutdown. On Racks at `3.25.10` or later, the countdown does not arm again unless spend crosses the new cap.
- **Cancel Shutdown:** Resets the budget state without changing the cap. If spend is still at or above the cap, the next accumulator tick, within 10 minutes, arms a new countdown.

### Active

Countdown expired. All eligible Services have been scaled to 0 replicas. The banner shows how many Services were affected and when shutdown occurred.

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
- Pre-fills with a suggested value (current cap x 1.5, rounded up to a multiple of $50)
- On Racks at `3.25.10` or later, resets the threshold and cap alerts, so each can fire again this month against the new cap
- If auto-shutdown is armed, raising above current spend cancels the scheduled shutdown

### Budget Reset

Resets the budget enforcement state:

- Re-enables normal operations and new deploys
- For Active (shutdown) state: restores Services to original replica counts with 24-hour cooldown
- For Armed state: cancels the scheduled shutdown
- If spend is still at or above the cap, the next accumulator tick, within 10 minutes, blocks deploys again or arms a new countdown. After a restore, the 24-hour cooldown holds off a new shutdown.
- The Budget Reset dialog leaves the 24-hour cooldown in place. To clear it, run `convox budget reset --force-clear-cooldown <app>` (Administrator role required), or use **Reset Period** in the Cost Breakdown section below, which also zeroes spend.

## Per-App Cost Breakdown

Below the budget configuration, the Cost Breakdown section displays per-Service spend with:

- **Service-level rows** showing instance type, capacity type (on-demand vs. spot), and accumulated cost
- **Date range filtering** in UTC days on Racks at `3.25.10` or later (see [Date ranges](#date-ranges)), and Service name filtering
- **Aggregate toggle** to group by Service or show individual breakdowns
- **Warning banner** when pods run on unpriced instance types
- **Reset Period** (organization Administrators only) to zero the App's month-to-date spend and start the period at the current time. It also does everything Budget Reset does and clears the 24-hour cooldown: it clears the breaker and restores Services that auto-shutdown scaled to zero. The period still rolls over on the 1st.

## See Also

- [Budget Caps (CLI Reference)](/management/budget-caps)
- [Cost Tracking](/management/cost-tracking)
- [GPU Dashboard](/console/gpu-dashboard)
- [Model Deploy Wizard](/console/deploy-wizard)
- [Service Detail](/console/service-detail)
