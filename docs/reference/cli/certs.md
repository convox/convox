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

### Flags

| Flag | Description |
| ---- | ----------- |
| `--generated` | List only generated certificates |

### Examples
```bash
    $ convox certs
    ID                                      DOMAIN             EXPIRES
    cert-0a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d  *.example.com      364 days from now
    cert-1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e  myapp.example.org  364 days from now
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
| `--duration` | Certificate duration (e.g., `4200h`) |
| `--issuer` | Certificate issuer (e.g., `letsencrypt`) |
| `--id` | Output only the certificate ID |

### Examples
```bash
    $ convox certs generate mydomain.com --duration 4200h --issuer letsencrypt
```

To list generated certificates:
```bash
    convox certs --generated
```

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

Renew a certificate for an app.

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

## See Also

- [SSL](/deployment/ssl) for SSL certificate configuration
- [Custom Domains](/deployment/custom-domains) for routing domains to services