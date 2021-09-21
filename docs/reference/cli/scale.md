---
title: "scale"
draft: false
slug: scale
url: /reference/cli/scale
---
# scale

## scale

Scale a service

### Usage
```html
    convox scale <service>
```
### Examples
```html
    $ convox scale web --count 3 --cpu 256 --memory 1024
    Scaling web...
    2020-01-22T14:54:50Z system/cloudformation aws/cfm test-nodejs UPDATE_IN_PROGRESS test-nodejs User Initiated
    2020-01-22T14:54:55Z system/cloudformation aws/cfm test-nodejs UPDATE_IN_PROGRESS ResourceDatabase
    2020-01-22T14:54:56Z system/cloudformation aws/cfm test-nodejs UPDATE_COMPLETE ResourceDatabase
    2020-01-22T14:54:59Z system/cloudformation aws/cfm test-nodejs UPDATE_IN_PROGRESS ServiceWeb
    ...
    2020-01-22T14:57:53Z system/cloudformation aws/cfm test-nodejs UPDATE_COMPLETE ServiceWeb
    2020-01-22T14:57:54Z system/cloudformation aws/cfm test-nodejs UPDATE_COMPLETE ResourceDatabase
    2020-01-22T14:57:54Z system/cloudformation aws/cfm test-nodejs UPDATE_COMPLETE test-nodejs
    OK
```