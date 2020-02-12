# ssl

## ssl

List certificate associates for an app

### Usage

    convox ssl

### Examples

    $ convox ssl
    ENDPOINT  CERTIFICATE       DOMAIN                                                       EXPIRES
    web:443   acm-81ea927349d6  *.test-router-ubbtc4og6b40-1320539375.us-east-1.convox.site  8 months from now

## ssl update

Update certificate for an app

### Usage

    convox ssl update <process:port> <certificate>

### Examples

    $ convox ssl update web:443 updated.crt