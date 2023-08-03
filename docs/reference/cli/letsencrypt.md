---
title: "apps"
draft: false
slug: apps
url: /reference/cli/letsencrypt
---
# letsencrypt

## letsencrypt dns route53 list

List configured letsencrypt dns solvers for route53

### Usage
```html
    convox letsencrypt dns route53 list
```
### Examples
```html
    $ convox letsencrypt dns route53 list
    ID  DNS-ZONES    HOSTED-ZONE-ID  REGION     ROLE
    1   convox.site  XXXXXXXXXXXXX   us-east-1  arn:aws:iam::XXXXXXXXXXXX:role/dns-access
```
## letsencrypt dns route53 update

Update letsencrypt dns solvers for route53 dns zone

### Usage
```html
    convox letsencrypt dns route53 update --id [id] --dns-zones [dnz-zone] --role [role] --hosted-zone-id [zone-id] --region [region]
```

### Examples
```html
    $ convox letsencrypt dns route53 update --id 1 --dns-zones xxxx --role arn:aws:iam::XXXXXXXXXXXX:role/dns-access --hosted-zone-id xxxxxxxxxxx --region us-east-1
    OK
```

## letsencrypt dns route53 delete

Delete letsencrypt dns solvers for route53 dns zone

### Usage
```html
    convox letsencrypt dns route53 delete --id [id]
```

### Examples
```html
    $ convox letsencrypt dns route53 delete --id 1
    OK
```

## letsencrypt dns route53 role

Letsencrypt route53 role

### Usage
```html
    convox letsencrypt dns route53 role
```

### Examples
```html
    $ convox letsencrypt dns route53 role
    arn:aws:iam::XXXXXXXXXXXX:role/xxx-cert-manager
```
