---
title: "registries"
slug: registries
url: /reference/cli/registries
---
# registries

## registries

List private registries

### Usage
```bash
    convox registries
```
### Examples
```bash
    $ convox registries
    SERVER                                        USERNAME
    123456789012.dkr.ecr.us-east-1.amazonaws.com  AKIAABCDE1F2GHIJKLMN
    private.registry.com                          my_private_name
    https://index.docker.io/v1/                           my_docker_name
    quay.io                                       my_quay_name
```
## registries add

Add a private registry

### Usage
```bash
    convox registries add <server> <username> <password>
```
### Examples
```bash
    $ convox registries add 123456789012.dkr.ecr.us-east-1.amazonaws.com AKIAABCDE1F2GHIJKLMN l0nG+4nD/c0mpl3X+p455w0RD
    Adding registry... OK
```

> Treat registry credentials with the same care as any other secret. Avoid committing them to version control.

## registries remove

Remove private registry

### Usage
```bash
    convox registries remove <server>
```
### Examples
```bash
    $ convox registries remove 123456789012.dkr.ecr.us-east-1.amazonaws.com
    Removing registry... OK
```

## See Also

- [Private Registries](/configuration/private-registries) for private registry configuration