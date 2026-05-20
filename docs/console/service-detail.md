---
title: "Service Detail"
slug: service-detail
url: /console/service-detail
---
# Service Detail

The Service detail page shows a single Service within an App, including resource allocation, scaling configuration, environment variables, GPU telemetry, live logs, events, and cost data.

## Navigating to the Service Detail Page

From the Console sidebar, select an App under a Rack. On the App page, click any Service name in the Services list to open its detail view.

The URL follows the pattern: `/<org>/<rack>/<app>/services/<service-name>`.

## Header

The header displays:

- **Service name** and current replica count (e.g., `3 / 5` for 3 running out of 5 max)
- **Agent badge** — shown when the Service runs as a DaemonSet (one replica per node)
- **Domain links** — clickable HTTPS links for each domain assigned to the Service
- **Resource cards** — CPU (millicores), Memory (MB), and GPU (count and vendor) at a glance
- **Refresh button** — forces a re-fetch of all Service data

## Panels

A tab bar below the header switches between panels. The active panel is reflected in the URL query string (`?section=logs`, `?section=scaling`, etc.) so links can deep-link to a specific panel. V2 Racks show only the Overview panel.

### Overview

The default panel. Four cards:

- **Identity** — App name, Service name, domains, current Release ID
- **Configuration** — CPU, memory, GPU allocation, min/max replicas, cold-start indicator
- **Recent activity** — last 3 events (deploy, restart, scale, override toggles)
- **Health** — App status badge, replica count, autoscale enabled/disabled, agent type

### Logs

Streams live log output from the Service. Controls:

- **Pause / Resume** — buffers incoming lines while paused and flushes them on resume
- **Clear** — clears the log terminal
- **Filter** — regex filter applied client-side to the displayed lines
- **Wrap toggle** — enables or disables line wrapping in the terminal

The log stream reconnects automatically (up to 3 retries with exponential backoff). A manual Retry button appears if all retries are exhausted.

### Events

A table of events scoped to this Service, including deploys, restarts, scale changes, and autoscale override actions. Each row shows the timestamp, event summary, actor, and status. A link at the bottom navigates to the full App events view.

### Scaling

Displays the Service scaling configuration and provides controls to change it.

**Bounds card** — shows min replicas, max replicas, and current replica count. A cold-start badge appears when the Service can scale to zero.

**Autoscale settings** — shows the current autoscale state:

| Badge | Meaning |
|-------|---------|
| Override active | Console-driven autoscaler is managing this Service |
| From convox.yml | Autoscale triggers are declared in the manifest |
| Not configured | No autoscaler is configured |

A trigger table displays thresholds for CPU, memory, GPU utilization, and inference queue depth. When an override is active, click the pencil icon on a threshold cell to edit the value inline.

**Actions card** — two scale modes:

- **Fixed count** — set a single replica count
- **Range (min-max)** — set a min and max for autoscaling

Both modes include a confirmation dialog for high-risk changes (scaling to 0 or large jumps). A **Restart** button performs a rolling restart of all replicas.

**Enable / Override / Disable autoscale** — use these buttons to manage Console-driven autoscale overrides. See [Autoscale Triggers](/console/autoscale-triggers) for details.

### Environment

Displays per-Service environment variable overrides defined in `convox.yml` under the `services.<name>.environment` block. Values matching sensitive key patterns (TOKEN, KEY, SECRET, PASSWORD) are masked by default with a reveal toggle.

This panel requires Rack version 3.24.6 or later.

### Cost

Shows month-to-date spend for this Service. Requires [cost tracking](/management/cost-tracking) to be enabled on the Rack.

- **Headline card** — total spend in USD with pricing table label
- **Breakdown table** — per instance type and capacity type (on-demand vs. spot), showing active replicas and spend

A link at the bottom navigates to the full App cost breakdown.

### GPU

Displays GPU telemetry for Services with GPU reservations. Requires GPU observability to be enabled on the Rack.

- **Summary cards** — GPU utilization, memory used/total, vendor and count
- **Extended counters** — tensor active, SM active, DRAM active, FP16/FP32 active, power draw
- **Utilization chart** — time-series GPU utilization with a configurable display window (5m, 30m, 1h, 24h)
- **Per-process table** — snapshot of each running GPU Process with utilization and memory stats
- **Grafana deep link** — when a Grafana URL is configured on the Rack, a button links to the per-Service dashboard

### Test Model

Available when the Service has a domain (including internal `.local` domains for private Services). Opens an interactive playground for sending requests to the Service endpoint.

## See Also

- [Autoscale Triggers](/console/autoscale-triggers) for managing Console-driven autoscaling
- [Cost Tracking](/management/cost-tracking) for enabling per-App spend tracking
- [Scaling](/configuration/scaling) for convox.yml scale configuration
