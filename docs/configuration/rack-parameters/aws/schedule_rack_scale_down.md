---
title: "schedule_rack_scale_down"
draft: false
slug: schedule_rack_scale_down
url: /configuration/rack-parameters/aws/schedule_rack_scale_down
---

# schedule_rack_scale_down

## Description
The `schedule_rack_scale_down` parameter specifies the schedule for scaling down the rack using the Unix cron syntax format. Example: '0 18 * * 5'. The supported cron expression format consists of five fields separated by white spaces: [Minute] [Hour] [Day_of_Month] [Month_of_Year] [Day_of_Week]. More details on the CRON format can be found in [Crontab](http://crontab.org/) and [examples](https://crontab.guru/examples.html). The time is calculated in **UTC**.

## Default Value
The default value for `schedule_rack_scale_down` is an empty string.

If the `schedule_rack_scale_down` parameter is set to an empty string, no scale-down schedule is applied by default. This parameter is optional and can be configured based on your specific needs.

## Use Cases
- **Cost Optimization**: Schedule the rack to scale down during off-peak hours to reduce costs.
- **Resource Management**: Ensure that resources are scaled down when not in use to optimize resource allocation.

## Setting Parameters
To set the `schedule_rack_scale_down` parameter, use the following command:
```html
$ convox rack params set schedule_rack_scale_down="0 18 * * 5" -r rackName
Setting parameters... OK
```
This command sets the rack scale down schedule to every Friday at 6 PM UTC.

## Additional Information
The `schedule_rack_scale_down` parameter is used in conjunction with the [schedule_rack_scale_up](/configuration/rack-parameters/aws/schedule_rack_scale_up) parameter. Both parameters must be properly configured to ensure that the rack scales up and down according to the desired schedule.

Properly scheduling the scale down times can help you optimize costs and resource usage. Ensure that the cron expressions are set correctly and consider the time zone differences when setting the schedule. For more information on configuring cron schedules, refer to the [Crontab documentation](http://crontab.org/).
