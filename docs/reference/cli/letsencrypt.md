---
title: "letsencrypt"
draft: false
slug: letsencrypt
url: /reference/cli/letsencrypt
---
# letsencrypt

## letsencrypt dns route53 list

List configured Let's Encrypt DNS solvers for Route53.

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

## letsencrypt dns cloudflare list

List configured Let's Encrypt DNS solvers for Cloudflare.

### Usage
```html
    convox letsencrypt dns cloudflare list
```
### Examples
```html
    $ convox letsencrypt dns cloudflare list
    ID  DNS-ZONES      TYPE       SECRET                         FIELDS          EMAIL
    1   example.com    API Token  cloudflare-dns-credential-1   api-token
```
Supplying `--api-token` or `--api-key` causes Convox to create or update a secret in the `cert-manager` namespace automatically (defaulting to `cloudflare-dns-credential-<id>` for the solver). `api-token` is written for token-based auth; API key auth stores both `api-key` and `email` entries in the same secret.
## letsencrypt dns route53 update

Update Let's Encrypt DNS solvers for a Route53 hosted zone.

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

Delete Let's Encrypt DNS solvers for a Route53 hosted zone.

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

Show the IAM role the rack uses for Route53 DNS-01 challenges.

### Usage
```html
    convox letsencrypt dns route53 role
```

### Examples
```html
    $ convox letsencrypt dns route53 role
    arn:aws:iam::XXXXXXXXXXXX:role/xxx-cert-manager

## letsencrypt dns cloudflare add

Configure a Let's Encrypt DNS solver that uses a Cloudflare API token or API key secret stored in Kubernetes.

### Usage
```html
    convox letsencrypt dns cloudflare add --id [id] --dns-zones [dns-zone] \
      [--api-token token | --api-key key --email email]
```

### Examples
```html
    $ convox letsencrypt dns cloudflare add --id 1 --dns-zones example.com \
        --api-token myCloudflareTokenValue
    OK

    $ convox letsencrypt dns cloudflare add --id 2 --dns-zones example.com \
        --api-key abc123 --email ops@example.com
    OK
```

## letsencrypt dns cloudflare update

Update an existing Cloudflare DNS solver. All provided flags replace the stored values for that solver ID.

### Usage
```html
    convox letsencrypt dns cloudflare update --id [id] [--dns-zones dns-zone] \
      [--api-token token | --api-key key --email email]
```

### Examples
```html
    $ convox letsencrypt dns cloudflare update --id 1 --dns-zones example.com \
        --api-token myNewCloudflareTokenValue
    OK
```

## letsencrypt dns cloudflare delete

Remove a Cloudflare DNS solver by ID.

### Usage
```html
    convox letsencrypt dns cloudflare delete --id [id]
```

### Examples
```html
    $ convox letsencrypt dns cloudflare delete --id 1
    OK
```
```
