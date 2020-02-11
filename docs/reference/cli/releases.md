# releases

## releases

List releases for an app

### Usage

    convox releases

### Examples

    $ convox releases
    ID          STATUS  BUILD        CREATED         DESCRIPTION
    RIABCDEFGH          BJABCDEFGHI  30 seconds ago
    RABCDEFGHI  active  BABCDEFGHIJ  2 weeks ago
    RBCDEFGHIJ          BBCDEFGHIJK  2 weeks ago

## releases info

Get information about a release

### Usage

    convox releases info <release>

### Examples

    $ convox releases info RABCDEFGHI
    Id           RABCDEFGHI
    Build        BABCDEFGHIJ
    Created      2020-01-22T15:37:38Z
    Description
    Env

## releases manifest

Get manifest for a release

### Usage

    convox releases manifest <release>

### Examples

    $ convox releases manifest RABCDEFGHI
    services:
        web:
            build: .
            port: 3000

## releases promote

Promote a release

### Usage

    convox releases promote <release>

### Examples

    $ convox releases promote RIABCDEFGH
    Promoting RIABCDEFGH...
    2020-02-11T20:55:37Z system/k8s/atom/app Status: Running => Pending
    2020-02-11T20:55:44Z system/k8s/web Scaled up replica set web-856bf5dbdf to 1
    2020-02-11T20:55:44Z system/k8s/web-856bf5dbdf-qkcm9 Successfully assigned convox-myapp/web-856bf5dbdf-qkcm9 to aks-default-22457946-vmss000000
    2020-02-11T20:55:44Z system/k8s/web-856bf5dbdf Created pod: web-856bf5dbdf-qkcm9
    2020-02-11T20:55:46Z system/k8s/web-856bf5dbdf-qkcm9 Pulling image "convoxctuntzfzqjho.azurecr.io/myapp:web.BJABCDEFGHI"
    2020-02-11T20:55:47Z system/k8s/web-856bf5dbdf-qkcm9 Successfully pulled image "convoxctuntzfzqjho.azurecr.io/myapp:web.BJABCDEFGHI"
    2020-02-11T20:55:48Z system/k8s/web-856bf5dbdf-qkcm9 Created container main
    2020-02-11T20:55:48Z system/k8s/web-856bf5dbdf-qkcm9 Started container main
    2020-02-11T20:55:54Z system/k8s/web Scaled down replica set web-7f58f4574 to 0
    2020-02-11T20:55:58Z system/k8s/atom/app Status: Pending => Updating
    2020-02-11T20:55:59Z system/k8s/atom/service/web Status: Running => Pending
    OK

## releases rollback

Copy an old release forward and promote it

### Usage

    convox releases rollback <release>

### Examples

    $ convox releases rollback RABCDEFGHI
    Rolling back to RABCDEFGHI... OK, RHIABCDEFG
    Promoting RHIABCDEFG...
    2020-02-11T20:58:01Z system/k8s/atom/app Status: Running => Pending
    2020-02-11T20:58:07Z system/k8s/web-95848bb45 Created pod: web-95848bb45-9fqts
    2020-02-11T20:58:07Z system/k8s/web-95848bb45-9fqts Successfully assigned convox-myapp/web-95848bb45-9fqts to aks-default-22457946-vmss000001
    2020-02-11T20:58:09Z system/k8s/web-95848bb45-9fqts Container image "convoxctuntzfzqjho.azurecr.io/myapp:web.BABCDEFGHIJ" already present on machine
    2020-02-11T20:58:09Z system/k8s/web-95848bb45-9fqts Created container main
    2020-02-11T20:58:10Z system/k8s/web-95848bb45-9fqts Started container main
    2020-02-11T20:58:14Z system/k8s/atom/app Status: Pending => Updating
    2020-02-11T20:58:20Z system/k8s/web-856bf5dbdf Deleted pod: web-856bf5dbdf-qkcm9
    2020-02-11T20:58:20Z system/k8s/web Scaled down replica set web-856bf5dbdf to 0
    2020-02-11T20:58:21Z system/k8s/atom/service/web Status: Running => Pending
    2020-02-11T20:58:33Z system/k8s/atom/service/web Status: Pending => Updating
    2020-02-11T20:58:33Z system/k8s/atom/app Status: Updating => Running
    2020-02-11T20:58:34Z system/k8s/atom/service/web Status: Updating => Running
    OK
