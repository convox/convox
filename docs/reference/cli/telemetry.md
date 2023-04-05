---
title: "telemetry"
draft: false
slug: telemetry   
url: /reference/cli/telemetry
---
# Telemetry

## set

Activate or Desactivate to send telemetry data to convox team. This configuration is used just for customers who has your own console-app. 
For customer who uses convox's console, won't be possible to desactivate this sending. 

### Usage
```html
    convox telemetry set true
```

### Examples
```html
    convox telemetry set true
    OK

    convox telemetry set false
    OK

    convox telemetry set blah
    ERROR: command accepts just 'true' or 'false' as argument
```