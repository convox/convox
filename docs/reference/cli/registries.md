---
title: "registries"
draft: false
slug: registries
url: /reference/cli/registries
---
# registries

## registries

List private registries

### Usage
```html
    convox registries
```
### Examples
```html
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
```html
    convox registries add <server> <username> <password>
```
### Examples
```html
    $ convox registries add 123456789012.dkr.ecr.us-east-1.amazonaws.com AKIAABCDE1F2GHIJKLMN l0nG+4nD/c0mpl3X+p455w0RD
    Adding registry... OK
```
## registries remove

Remove private registry

### Usage
```html
    convox registries remove <server>
```
### Examples
```html
    $ convox registries remove 123456789012.dkr.ecr.us-east-1.amazonaws.com
    Removing registry... OK
```