# registries

## registries

List private registries

### Usage

    convox registries

### Examples

    $ convox registries
    SERVER                                        USERNAME
    123456789012.dkr.ecr.us-east-1.amazonaws.com  AKIAABCDE1F2GHIJKLMN
    private.registry.com                          my_private_name
    index.docker.io/v1/                           my_docker_name
    quay.io                                       my_quay_name

## registries add

Add a private registry

### Usage

    convox registries add <server> <username> <password>

### Examples

    $ convox registries add 123456789012.dkr.ecr.us-east-1.amazonaws.com AKIAABCDE1F2GHIJKLMN l0nG+4nD/c0mpl3X+p455w0RD
    Adding registry... OK

## registries remove

Remove private registry

### Usage

    convox registries remove <server>

### Examples

    $ convox registries remove 123456789012.dkr.ecr.us-east-1.amazonaws.com
    Removing registry... OK