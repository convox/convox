---
title: "registry_burst"
draft: false
slug: registry_burst
url: /configuration/rack-parameters/aws/registry_burst
---

# registry_burst

## Description
The `registry_burst` parameter defines the maximum number of image pull requests that can be made in a burst, exceeding the `registry_pull_qps` limit for a short duration. This parameter allows for short-lived spikes in image pull traffic.

## Default Value
The default value for `registry_burst` is `10`.

## Use Cases
- **Handle burst traffic**: Allow for temporary spikes in image pull requests without exceeding the registry_pull_qps limit.
- **Improve pod startup time**: Permit a higher initial burst of image pulls to accelerate pod startup.

## Setting Parameters
To enable the `registry_burst` parameter, use the following command:
```html
$ convox rack params set registry_burst=value -r rackName
Setting parameters... OK
```

Replace value with the desired maximum number of burst image pull requests.

## Additional Information
The `registry_burst` parameter complements `registry_pull_qps` by providing flexibility in handling short-lived spikes in image pull traffic. However, excessive burst values can still overload the registry. It's essential to consider the average image pull rate and the expected peak load when setting this value.


