---
title: "instances"
draft: false
slug: instances
url: /reference/cli/instances
---
# instances

## instances

List instances

### Usage
```html
    convox instances
```
### Examples
```html
    $ convox instances
    ID                   STATUS  STARTED       PS  CPU     MEM     PUBLIC          PRIVATE
    i-029382969778a743a  active  2 months ago  3   18.75%  45.08%  32.207.218.250  10.0.2.39
    i-06d0eaf588c96ee5a  active  2 months ago  2   18.75%  32.64%  52.208.102.198  10.0.2.17
    i-0a69dd90d3b542c3a  active  2 months ago  3   21.88%  58.13%  52.160.141.135  10.0.1.151
    i-0cbaa6d2dd1d094ca  active  2 months ago  5   37.50%  77.72%  1.226.241.132   10.0.3.45
    i-0d4493dded1fa9aea  active  2 months ago  5   50.00%  97.91%  52.144.245.283  10.0.1.56
```
## instances terminate

Terminate an instance

### Usage
```html
    convox instances terminate <instance_id>
```
### Examples
```html
    $ convox instances terminate i-029382969778a743a
    Terminating instance... OK
```

## instances keyroll

Roll ssh key on instances

### Usage
```html
    convox instances keyroll
```
For v3 rack it will generate a private key and show it as output. Please save the generated private key and use it when sshing into an instance
### Examples
```html
    $ convox instances keyroll
    Rolling instance key... OK
    Updating parameters... OK
    Generated private key:
    -----BEGIN RSA PRIVATE KEY-----
    MIIE...
    -----END RSA PRIVATE KEY-----
```

## instances ssh

Run a shell on an instance

### Usage
**For v2 rack:**
```html
    convox instances ssh <instance_id>
```

**For v3 rack:**
```html
    convox instances ssh <instance_id> --key <private_key_file>
```

### Examples
```html
    $ convox instances ssh ip-10-1-80-201.ec2.internal --key ~/.ssh/rack/priv.pem
```
