---
title: "login"
slug: login
url: /reference/cli/login
---
# login

The `convox login` command authenticates your CLI against a Convox Console. This is required before running most other commands. For CI/CD pipelines, you can set the `CONVOX_HOST` and `CONVOX_PASSWORD` environment variables instead of running `login` interactively.

## login

Authenticate your CLI with a Console installation

### Usage
```bash
    convox login [hostname]
```
### Examples
```bash
    $ convox login console.convox.com
    Authenticating with console.convox.com... OK

    $ convox login console.convox.com -t a1234567-acde-1234-abcde-123abc456def
    Authenticating with console.convox.com... OK
```