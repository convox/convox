---
title: "scale"
slug: scale
url: /reference/cli/scale
---
# scale

## scale

Scale a service

### Usage
```bash
    convox scale <service>
```
### Examples
```bash
    $ convox scale web --count 3 --cpu 256 --memory 1024
    Scaling web...
    2026-01-15T14:54:50Z system/cloudformation aws/cfm test-nodejs UPDATE_IN_PROGRESS test-nodejs User Initiated
    2026-01-15T14:54:55Z system/cloudformation aws/cfm test-nodejs UPDATE_IN_PROGRESS ResourceDatabase
    2026-01-15T14:54:56Z system/cloudformation aws/cfm test-nodejs UPDATE_COMPLETE ResourceDatabase
    2026-01-15T14:54:59Z system/cloudformation aws/cfm test-nodejs UPDATE_IN_PROGRESS ServiceWeb
    ...
    2026-01-15T14:57:53Z system/cloudformation aws/cfm test-nodejs UPDATE_COMPLETE ServiceWeb
    2026-01-15T14:57:54Z system/cloudformation aws/cfm test-nodejs UPDATE_COMPLETE ResourceDatabase
    2026-01-15T14:57:54Z system/cloudformation aws/cfm test-nodejs UPDATE_COMPLETE test-nodejs
    OK
```