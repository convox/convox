# App

An App is a logical container for [Primitives](../README.md) that are updated together through transactional deployments.

## Definition

An App is defined by a single [`convox.yml`](../../../configuration/convox.yml.md)

    resources:
      database:
        type: postgres
    services:
      web:
        build: .
        resources:
          - database

## Command Line Interface

### Creating an App

    $ convox apps create myapp
    Creating myapp... OK

### Getting information about an App

    $ convox apps info myapp
    Name    myapp
    Status  running
    Locked  false
    Release RABCDEFGHI
    Router  router.0a1b2c3d4e5f.convox.cloud

### Listing Apps

    $ convox apps
    APP    STATUS   RELEASE
    myapp  running  RABCDEFGHI

### Deleting an App

    $ convox apps delete myapp
    Deleting myapp... OK

### Getting logs for an App

    $ convox logs -a myapp
    2000-01-01T00:00:00 service/web/web-zyxwv Starting myapp on port 5000

### Cancelling a deployment that is in progress

    $ convox apps cancel myapp
    Cancelling deployment of myapp... OK

### Preventing accidental deletion of an App

    $ convox apps lock myapp
    Locking myapp... OK

    $ convox apps unlock myapp
    Unlocking myapp... OK

### Exporting an App

    $ convox apps export myapp -f /tmp/myapp.tgz
    Exporting app myapp... OK
    Exporting env... OK
    Exporting build BABCDEFGHI... OK
    Exporting resource database... OK
    Packaging export... OK

### Importing an App

    $ convox apps import myapp2 -f /tmp/myapp.tgz
    Creating app myapp2... OK
    Importing build... OK, RIHGFEDCBA
    Importing env... OK, RJIHGFEDCB
    Promoting RJIHGFEDCB... OK
    Importing resource database... OK
