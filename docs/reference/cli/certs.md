## certs

List certificates

### Usage

    convox certs

### Examples

    $ convox certs
    ID                          DOMAIN                                                       EXPIRES
    acm-a123b45678c9            *.custo-route-12abcdefghij3-123456789.us-east-1.convox.site  1 year from now
    acm-d456e78901f2            *.test-router-abcde1ef2g34-1234567890.us-east-1.convox.site  8 months from now
    acm-g789h01234i5            *.softwarebalance.com                                        10 months from now
    cert-test-1234567890-12345  *.*.elb.amazonaws.com                                        8 months from now

## certs delete

Delete a certificate

### Usage

    convox certs delete <cert>

### Examples

    $ convox certs delete acm-1234abcd5678
    Deleting certificate acm-1234abcd5678... OK

## certs generate

Generate a certificate

### Usage

    convox certs generate <domain> [domain...]

### Examples

    $ convox certs generate *.domain.com
    Generating certificate... OK, acm-1234abcd5678

## certs import

Import a certificate

### Usage

    convox certs import <pub> <key>

### Examples

    $ convox certs import cert.pub cert.key
    Importing certificate... OK, cert-test-1234567890-12345