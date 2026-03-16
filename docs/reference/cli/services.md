---
title: "services"
slug: services
url: /reference/cli/services
---
# services

## services

List services for an app

### Usage
```bash
    convox services
```
### Examples
```bash
    $ convox services 
    SERVICE  DOMAIN                                                                PORTS
    web      nodejs-web.test-Router-ABCDEF0123456-1234567890.us-east-1.convox.site  80:3000 443:3000
```
## services restart

Restart a service

### Usage
```bash
    convox services restart <service>
```
### Examples
```bash
    $ convox services restart web
    Restarting web... OK
```