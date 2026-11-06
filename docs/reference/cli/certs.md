---
title: "certs"
description: "The convox certs command lists, generates, imports, renews, and deletes SSL certificates, and moves V2 Rack certificates to DNS validation."
slug: certs
url: /reference/cli/certs
---
# certs

## certs

List certificates.

### Usage
To list all certificates

```bash
    convox certs
```

### Flags

| Flag | Description |
| ---- | ----------- |
| `--generated` | List only generated certificates |

### Examples
```bash
    $ convox certs
    ID                                     DOMAIN             EXPIRES            Status
    cert-0a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d  *.example.com      364 days from now  Ready
    cert-1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e  myapp.example.org  364 days from now  Ready
```

On a V3 Rack, the `Status` column shows `Ready` for issued certificates and `Not Ready` for generated certificates that are still pending issuance.

On a V2 Rack on version 20261021234916 or later, the `Status` column shows how ACM validates each certificate. Older V2 Racks show `Ready`.

| Status | Meaning |
| ------ | ------- |
| `dns` | Issued and DNS validated; ACM renews it while its validation record stays in place |
| `email` | Issued and email validated; renewals stop on 2027-09-30 |
| `migrating` | Switching to DNS validation |
| `pending` | Not issued yet |
| `failed` | ACM validation failed or timed out |
| `imported` | Imported certificate; validation does not apply |

With CLI 3.25.12 or later, a closing line follows the table when certificates the Rack uses are still on email validation:

- `1 certificate uses email validation, which stops renewing on 2027-09-30: run convox certs dns` (`N certificates use` for more than one)
- `1 convox.site certificate is still on email validation: run convox certs migrate to see why` (`N convox.site certificates are still` for more than one)

```bash
    $ convox certs
    ID                DOMAIN                                           EXPIRES            Status
    acm-89ea927329d7  *.myrack-router-123456789.us-east-1.convox.site  5 months from now  dns
    acm-0a1b2c3d4e5f  example.com                                      3 months from now  email
    acm-1b2c3d4e5f6a  shop.example.org                                 4 months from now  email

    2 certificates use email validation, which stops renewing on 2027-09-30: run convox certs dns
```

## certs generate

Generate certificates. These certificates can be reused with convox apps. For example, generating a wildcard certificate to reuse it in several apps to reduce letsencrypt rate limit issue.

### Usage
To generate one or more certificates

```bash
    convox certs generate <domain> [domain...]
```

### Flags

| Flag | Description |
| ---- | ----------- |
| `--duration` | Certificate duration (e.g., `4200h`). V3 Racks only |
| `--issuer` | Certificate issuer. The only accepted value is `letsencrypt`. V3 Racks only |
| `--id` | Output only the certificate ID |

On a V3 Rack without `--issuer`, the Rack creates a self-signed certificate for exactly one domain. With `--issuer letsencrypt`, it requests a Let's Encrypt certificate for up to 10 domains.

### Examples
```bash
    $ convox certs generate mydomain.com --duration 4200h --issuer letsencrypt
    Generating certificate... OK, cert-0a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b
```

Certificates generated on Rack version `3.25.7` and later reuse their private key across renewals. See [Private Key Reuse on Renewal](/deployment/ssl#private-key-reuse-on-renewal).

To list generated certificates:
```bash
    convox certs --generated
```

### V2 Racks

With CLI 3.25.12 or later and a V2 Rack on version 20261021234916 or later, `convox certs generate` requests an ACM certificate with DNS validation and prints the records to add. Deploy the domain once `convox certs` shows the certificate as `dns`.

```bash
    $ convox certs generate www.example.org
    Generating certificate... OK, acm-2c3d4e5f6a7b
    ZONE         NAME                                                TYPE   VALUE                                                             CHECK
    example.org  _2123456789abcdef0123456789abcdef.www.example.org.  CNAME  _ddcba9876543210fedcba9876543210.cdefghijkl.acm-validations.aws.  missing
    Deploy the domain once convox certs shows acm-2c3d4e5f6a7b as dns
```

If the records are not available yet, the command prints `Run convox certs dns in a minute to see the records to add` instead of the table. With `--id`, only the certificate ID goes to stdout. `--issuer` and `--duration` apply to V3 Racks only.

## certs delete

Delete a certificate.

### Usage
```bash
    convox certs delete <id>
```

### Examples
```bash
    $ convox certs delete cert-0a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d
    Deleting certificate cert-0a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d... OK
```

For a certificate generated with `--issuer letsencrypt`, this deletes the certificate's Secret. The Rack continues to renew that certificate and reissues the Secret, so the certificate reappears in `convox certs --generated`. Imported certificates and self-signed generated certificates are Secrets only, so deleting them removes them.

## certs import

Import a certificate.

### Usage
```bash
    convox certs import <pub> <key>
```

### Flags

| Flag | Description |
| ---- | ----------- |
| `--chain` | Path to an intermediate certificate chain file |
| `--id` | Output only the certificate ID |

### Examples
```bash
    $ convox certs import cert.pem key.pem
    Importing certificate... OK, cert-0a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d

    $ convox certs import cert.pem key.pem --chain chain.pem
    Importing certificate... OK, cert-0a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d
```

## certs renew

Trigger re-issuance of the certificates covering an App's custom domains. Certificates for Convox-generated hostnames are not affected.

### Usage
```bash
    convox certs renew
```

### Flags

| Flag | Short | Description |
| ---- | ----- | ----------- |
| `--app` | `-a` | App name (inferred from current directory if not specified) |

### Examples
```bash
    $ convox certs renew --app myapp
    Renewing certificate myapp... OK
```

## certs dns

List the DNS validation records for the certificates a V2 Rack uses, other than its `convox.site` certificates, and check whether each record resolves. Applies to V2 Racks only.

> Requires CLI 3.25.12 or later and a V2 Rack on version 20261021234916 or later. On a V3 Rack or an older V2 Rack, the command prints `ERROR: convox certs dns requires a rack on the latest release` and exits 1.

### Usage
```bash
    convox certs dns
```

### Flags

| Flag | Description |
| ---- | ----------- |
| `--zone-file` | Print the records as zone file lines on stdout. The lines about certificates still being prepared and per-certificate errors go to stderr |

### Examples
```bash
    $ convox certs dns
    ZONE         NAME                                                 TYPE   VALUE                                                              CHECK    CERTIFICATES
    example.com  _0123456789abcdef0123456789abcdef.example.com.       CNAME  _fedcba9876543210fedcba9876543210.abcdefghij.acm-validations.aws.  missing  acm-0a1b2c3d4e5f
    example.org  _1123456789abcdef0123456789abcdef.shop.example.org.  CNAME  _eedcba9876543210fedcba9876543210.bcdefghijk.acm-validations.aws.  ok       acm-1b2c3d4e5f6a

    2 records: 1 ok, 1 missing
    example.com is a Route 53 zone in this AWS account. To have the rack add its records: convox rack params set Route53CertificateValidation=Yes
    When every record shows ok: convox certs migrate --wait

    $ convox certs dns --zone-file
    ; zone example.com
    _0123456789abcdef0123456789abcdef.example.com. 300 IN CNAME _fedcba9876543210fedcba9876543210.abcdefghij.acm-validations.aws.
    ; zone example.org
    _1123456789abcdef0123456789abcdef.shop.example.org. 300 IN CNAME _eedcba9876543210fedcba9876543210.bcdefghijk.acm-validations.aws.
```

| Check | Meaning |
| ----- | ------- |
| `ok` | The name resolves to exactly this value |
| `missing` | No record exists at that name |
| `wrong` | Another value answers at that name, often from a wildcard record; add this record |
| `unknown` | The check did not finish in time; run the command again |

After the table, the command prints the number of records with a count for each check, then each of these lines that applies:

| Line | Printed when |
| ---- | ------------ |
| `<zone> is a Route 53 zone in this AWS account. To have the rack add its records: convox rack params set Route53CertificateValidation=Yes` | `Route53CertificateValidation` is off and the zone is a Route 53 hosted zone in the Rack's AWS account |
| `N certificates are still being prepared, run again in a few minutes` (`1 certificate is still being prepared` for one) | Some certificates' records are not available yet |
| `<id>: <error>` | Getting a certificate's records failed |
| `<id>, <id> use email validation but are not used by this rack, so it leaves them alone` (`<id> uses ... so it leaves it alone` for one) | Email-validated certificates in the account are not used by this Rack |
| `When every record shows ok: convox certs migrate --wait` | Any record is not `ok` |

## certs migrate

Switch certificates on a V2 Rack from email validation to DNS validation. A certificate switches only when every one of its records resolves. Applies to V2 Racks only.

> Requires CLI 3.25.12 or later and a V2 Rack on version 20261021234916 or later. On a V3 Rack or an older V2 Rack, the command prints `ERROR: convox certs migrate requires a rack on the latest release` and exits 1.

### Usage
```bash
    convox certs migrate [cert...]
```

With no arguments, the command covers every certificate the Rack uses.

### Flags

| Flag | Short | Description |
| ---- | ----- | ----------- |
| `--wait` | `-w` | Wait for the certificates whose result is `switching`, `publishing` or `preparing` to reach `dns`, checking every 10 seconds for at most 15 minutes |

With `--wait`, if any of those certificates does not reach `dns`, the command prints their results and exits 1 with `ERROR: certificates did not finish switching: <ids>`.

| Result | Meaning |
| ------ | ------- |
| `switching` | Every record resolves; the switch to DNS validation has started |
| `dns` | DNS validated |
| `waiting for its DNS record (run convox certs dns)` | A record is missing, wrong or could not be checked; nothing changed |
| `publishing` | The record was sent for a `convox.site` certificate, or written to Route 53, and does not resolve yet |
| `preparing` | The certificate's records are not available yet |
| `pending` | Not issued yet |
| `failed` | ACM validation failed or timed out |
| `imported` | Imported certificate; validation does not apply |
| `error: <detail>` | Anything else, for example `not used by this rack` or `certificate not found` |

### Examples
```bash
    $ convox certs migrate --wait
    CERTIFICATE       DOMAIN            RESULT
    acm-0a1b2c3d4e5f  example.com       waiting for its DNS record (run convox certs dns)
    acm-1b2c3d4e5f6a  shop.example.org  switching
    Waiting for switches to finish... OK
    CERTIFICATE       DOMAIN            RESULT
    acm-1b2c3d4e5f6a  shop.example.org  dns
```

## See Also

- [SSL](/deployment/ssl) for SSL certificate configuration
- [Custom Domains](/deployment/custom-domains) for routing domains to services