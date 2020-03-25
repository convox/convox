# Rack

A Rack is a platform to build, deploy and host your [Apps](../app).  It runs either locally on your own machine for development purposes or within your cloud infrastructure.

## Command Line Interface

### Installing a Rack

    $ convox rack install <provider> <name> [option=value]...

Where provider is currently one of `aws`, `azure`, `do`, `gcp`, or `local`.

    $ convox rack install aws myrack
    Upgrading modules...
    Downloading github.com/convox/convox?ref=3.0.11 for system...
    ...
    Apply complete! Resources: 49 added, 0 changed, 0 destroyed.

    Outputs:

    api = https://convox:password@api.1234567890abcdef.convox.cloud
    provider = aws

### Getting information about a Rack

    $ convox rack -r myrack
    Name      myrack
    Provider  aws
    Router    router.1234567890abcdef.convox.cloud
    Status    running
    Version   3.0.11

    $ convox rack params -r myrack

### Configuring a Rack

    $ convox rack params set node_type=t3.medium
    Updating parameters... Upgrading modules...
    Downloading github.com/convox/convox?ref=3.0.11 for system...
    ...

### Updating a Rack

    $ convox rack update -r myrack
    Upgrading modules...
    Downloading github.com/convox/convox?ref=3.0.11 for system...
    ...
    Apply complete! Resources: 0 added, 12 changed, 0 destroyed.

    Outputs:

    api = https://convox:password@api.1234567890abcdef.convox.cloud
    provider = aws

### Retrieving Rack logs

    $ convox rack logs -r myrack
    ...

### Uninstalling a Rack

    $ convox rack uninstall myrack

