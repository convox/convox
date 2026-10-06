---
title: "cost_tracking_history_days"
description: "The cost_tracking_history_days Azure rack parameter sets how many days of daily App cost history the Rack keeps for date-range queries, defaulting to 62."
slug: cost_tracking_history_days
url: /configuration/rack-parameters/azure/cost_tracking_history_days
---

# cost_tracking_history_days

## Description

The `cost_tracking_history_days` parameter sets how many days of daily cost history the Rack keeps for each App. While [`cost_tracking_enable`](/configuration/rack-parameters/azure/cost_tracking_enable) is `true`, the Rack records each tick's spend against the UTC day it falls in, per Service, and [`convox cost --start` and `--end`](/reference/cli/cost#date-ranges), with CLI version `3.25.10` or later, and the Console date range pickers sum those days. The current UTC day counts as one of them.

Month-to-date spend, budget caps, alerts and auto-shutdown do not use the history. Month-to-date spend still resets on the 1st, UTC.

## Default Value

The default value is `62`, which covers the current and previous calendar month.

## Accepted Values

An integer from `31` to `400`. The CLI rejects any other value with `cost_tracking_history_days must be an integer from 31 to 400`.

The parameter cannot be cleared. Set it to `62` to return to the default.

## Use Cases

- **Quarterly or yearly reports**: keep up to 400 days so `convox cost --start` can reach back across a full year.
- **Smaller history**: keep `31` days when only recent spend matters.

## Setting the Parameter

```bash
$ convox rack params set cost_tracking_history_days=120 -r rackName
Updating parameters... OK
```

The change updates the Rack API Deployment and creates no cloud resources.

Lowering the value drops the older days. Raising it keeps more days from then on; days already dropped do not come back.

## Viewing Current Configuration

```bash
$ convox rack params -r rackName
```

## Additional Information

- History starts at the first tick after the Rack updates to `3.25.10`. Nothing is backfilled, so the update day holds only part of that day's spend.
- The parameter has no effect while `cost_tracking_enable` is `false`. No ticks run, so no days are recorded.
- Requires Rack version `3.25.10` or later. An older CLI rejects the parameter on Azure with `unknown parameter 'cost_tracking_history_days' for azure provider`.
- Downgrading below `3.25.10` removes the parameter with `NOTICE: removing parameters not supported by version <version>: cost_tracking_history_days` on stderr. The stored history is kept, and no days are recorded while the Rack runs the older version, so upgrading again leaves a gap for that period. A Rack managed through the Console keeps the stored value and applies it again on the upgrade. A self-managed Rack drops the value, so set it again after upgrading, or days older than 62 are dropped.

## See Also

- [cost_tracking_enable](/configuration/rack-parameters/azure/cost_tracking_enable) for turning on cost tracking
- [cost](/reference/cli/cost#date-ranges) for `convox cost --start` and `--end`
- [Cost Tracking](/management/cost-tracking#daily-history-and-date-ranges) for how daily history is recorded
- [Budget Management](/console/budget-management) for the Console date range pickers
