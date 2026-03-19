---
title: "letsencrypt"
slug: letsencrypt
url: /reference/cli/letsencrypt
---
# letsencrypt

> These commands are currently available for AWS racks using Route53 for DNS management.

## letsencrypt dns route53 list

List configured letsencrypt dns solvers for route53

### Usage
```bash
    convox letsencrypt dns route53 list
```
### Examples
```bash
    $ convox letsencrypt dns route53 list
    ID  DNS-ZONES    HOSTED-ZONE-ID  REGION     ROLE
    1   example.com  XXXXXXXXXXXXX   us-east-1  arn:aws:iam::XXXXXXXXXXXX:role/dns-access
```
## letsencrypt dns route53 update

Update letsencrypt dns solvers for route53 dns zone

### Usage
```bash
    convox letsencrypt dns route53 update
```

### Flags

| Flag | Description |
| ---- | ----------- |
| `--id` | DNS solver ID |
| `--dns-zones` | Comma-separated DNS zones |
| `--hosted-zone-id` | Route53 hosted zone ID |
| `--role` | AWS IAM role ARN to assume for DNS access |
| `--region` | AWS region |

### Examples
```bash
    $ convox letsencrypt dns route53 update --id 1 --dns-zones xxxx --role arn:aws:iam::XXXXXXXXXXXX:role/dns-access --hosted-zone-id xxxxxxxxxxx --region us-east-1
    OK
```

## letsencrypt dns route53 delete

Delete letsencrypt dns solvers for route53 dns zone

### Usage
```bash
    convox letsencrypt dns route53 delete
```

### Flags

| Flag | Description |
| ---- | ----------- |
| `--id` | DNS solver ID to delete |

### Examples
```bash
    $ convox letsencrypt dns route53 delete --id 1
    OK
```

## letsencrypt dns route53 role

Letsencrypt route53 role

### Usage
```bash
    convox letsencrypt dns route53 role
```

### Examples
```bash
    $ convox letsencrypt dns route53 role
    arn:aws:iam::XXXXXXXXXXXX:role/xxx-cert-manager
```

## See Also

- [SSL](/deployment/ssl) for certificate management and DNS-01 configuration
- [Custom Domains](/deployment/custom-domains) for wildcard domains
- [certs](/reference/cli/certs) for managing certificates
