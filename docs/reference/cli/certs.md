---
title: "certs"
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

To list generated certificates:
```bash
    convox certs --generated
```

### Examples
```bash
    $ convox certs
    ID                                      DOMAIN             EXPIRES
    cert-0a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d  *.example.com      364 days from now
    cert-1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e  myapp.example.org  364 days from now
```

## certs generate

Generate certificates. These certificate can be reused with convox apps. For example, generating a wildcard certificate to reuse it in several apps to reduce letsencrypt rate limit issue.

### Usage
To list all certificates

```bash
    convox certs generate <domain> [domain...]
```

Option flags:
- `--duration`: Duration of the certificate.
- `--issuer`: Certificate issuer. For example: `letsencrypt`

### Examples
```bash
    $ convox certs generate mydomain.com --duration 4200h --issuer letsencrypt
```

To list generated certificates:
```bash
    convox certs --generated
```