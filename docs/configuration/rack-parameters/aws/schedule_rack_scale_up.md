---
title: "schedule_rack_scale_up"
draft: false
slug: schedule_rack_scale_up
url: /configuration/rack-parameters/aws/schedule_rack_scale_up
---

# schedule_rack_scale_up

## Description
The `schedule_rack_scale_up` parameter specifies the schedule for scaling up the rack using the Unix cron syntax format. Example: '0 0 * * 0'. The supported cron expression format consists of five fields separated by white spaces: [Minute] [Hour] [Day_of_Month] [Month_of_Year] [Day_of_Week]. More details on the CRON format can be found in [Crontab](http://crontab.org/) and [examples](https://crontab.guru/examples.html). The time is calculated in **UTC**.

## Default Value
The default value for `schedule_rack_scale_up` is `null`.

If the `schedule_rack_scale_up` parameter is set to `null`, no scale-up schedule is applied by default. This parameter is optional and can be configured based on your specific needs.

## Use Cases
- **Performance Optimization**: Schedule the rack to scale up during peak hours to ensure optimal performance.
- **Resource Management**: Ensure that resources are scaled up when needed to handle increased workloads.

## Managing Parameters

### Viewing Current Parameters
```html
$ convox rack params -r rackName
schedule_rack_scale_up  
```

### Setting Parameters
To set the `schedule_rack_scale_up` parameter, use the following command:
```html
$ convox rack params set schedule_rack_scale_up="0 0 * * 0" -r rackName
Setting parameters... OK
```
This command sets the rack scale up schedule to every Sunday at midnight UTC.

## Additional Information
The `schedule_rack_scale_up` parameter is used in conjunction with the [schedule_rack_scale_down](/configuration/rack-parameters/aws/schedule_rack_scale_down) parameter. Both parameters must be properly configured to ensure that the rack scales up and down according to the desired schedule.

Properly scheduling the scale up times can help you optimize performance and resource usage. Ensure that the cron expressions are set correctly and consider the time zone differences when setting the schedule. For more information on configuring cron schedules, refer to the [Crontab documentation](http://crontab.org/).
