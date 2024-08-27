---
title: "certs"
draft: false
slug: certs
url: /reference/cli/certs
---
# certs

## certs

List certificates.

### Usage
To list all certificates

```html
    convox certs
```

To list generated certificates:
```html
    convox certs --generated
```

### Examples
```html
    $ convox certs
```

## certs generate

Generate certificates. These certificate can be reused with convox apps. For example, generating a wildcard certificate to reuse it in several apps to reduce letsencrypt rate limit issue.

### Usage
To list all certificates

```html
    convox certs generate <domain> [domain...]
```

Option flags:
- `--duration`: Duration of the certificate.                      
- `--issuer`: Certificate issuer. For example: `letsencrypt`                          

### Examples
```html
    $ convox certs generate mydomain.com --duration 4200h --issuer letsencrypt
```

To list generated certificates:
```html
    convox certs --generated
```