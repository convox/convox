---
title: "build_node_min_count"
draft: false
slug: build_node_min_count
url: /configuration/rack-parameters/aws/build_node_min_count
---

# build_node_min_count

## Description
The `build_node_min_count` parameter specifies the minimum number of build nodes to keep running. If set to `0`, a build node will scale up when a build starts and will remain active until it has been idle for 30 minutes before scaling down.

## Default Value
The default value for `build_node_min_count` is `0`.

## Use Cases
- **Consistent Build Availability**: Ensures that there are always a minimum number of build nodes available to handle build tasks, reducing wait times and improving efficiency.
- **Performance Optimization**: Prevents delays in build processes, especially during peak development times, by maintaining a ready pool of build nodes.

## Managing Parameters

### Viewing Current Parameters
```html
$ convox rack params -r rackName
build_node_min_count  0
node_disk  20
node_type  t3.small
```

### Setting Parameters
To set the `build_node_min_count` parameter, use the following command:
```html
$ convox rack params set build_node_min_count=2 -r rackName
Setting parameters... OK
```
This command sets the minimum number of build nodes to 2.

## Additional Information
Adjusting the `build_node_min_count` allows you to manage the availability and readiness of your build infrastructure. Ensure that the value you set aligns with your team's build frequency and requirements. Keeping a higher minimum count can improve build times but will incur additional costs.

When `build_node_min_count` is set to `0`, a build node is automatically created at the start of a build and will remain active until it has been idle for 30 minutes before shutting down to conserve resources.
