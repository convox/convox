# rack resources

## rack resources

List resources

### Usage

    convox rack resources

### Examples

    $ convox rack resources 
    NAME                  TYPE     STATUS
    console-175092fe6ab1  webhook  running
    syslog-2984           syslog   running

## rack resources create

Create a resource

### Usage

    convox rack resources create <type> [Option=Value]...

### Examples

    $ convox rack resources create syslog Url=tcp+tls://www.domain.com:12345
    Creating resource... OK, syslog-3626

## rack resources delete

Delete a resource

### Usage

    convox rack resources delete <name>

### Examples

    $ convox rack resources delete syslog-3626
    Deleting resource... OK

## rack resources info

Get information about a resource

### Usage

    convox rack resources info <resource>

### Examples

    $ convox rack resources info syslog-2984
    Name     syslog-2984
    Type     syslog
    Status   running
    Options  Format=123457890abcdef1234567890 <22>1 {DATE} {GROUP} {SERVICE} {CONTAINER} - - [metas ddsource="{GROUP}" ddtags="container_id:{CONTAINER}"] {MESSAGE}
             Url=tcp+tls://intake.logs.datadoghq.com:10516
    URL      tcp+tls://intake.logs.datadoghq.com:10516

## rack resources link

Link a resource to an app

### Usage

    convox rack resources link <resource>

### Examples

    $ convox rack resources link syslog-2984 -a myapp
    Linking to myapp... OK

## rack resources options

List options for a resource type

### Usage

    convox rack resources options <resource>

### Examples

    $ convox rack resources options syslog
    NAME    DEFAULT                                                   DESCRIPTION
    Format  <22>1 {DATE} {GROUP} {SERVICE} {CONTAINER} - - {MESSAGE}  Syslog format string
    Url                                                               Syslog URL, e.g. 'tcp+tls://logs1.papertrailapp.com:11235'

    $ convox rack resources options webhook
    NAME  DEFAULT  DESCRIPTION
    Url            Webhook URL

## rack resources proxy

Proxy a local port to a rack resource

### Usage

    convox rack resources proxy <resource>

### Examples

    $ convox rack resources proxy syslog-2984
    proxying localhost:10516 to intake.logs.datadoghq.com:10516

## rack resources types

List resource types

### Usage

    convox rack resources types

### Examples

    $ convox rack resources types
    TYPE
    memcached
    mysql
    postgres
    redis
    s3
    sns
    sqs
    syslog
    webhook

## rack resources update

Update resource options

### Usage

    convox rack resources update <name> [Option=Value]...

### Examples

    $ convox rack resources update syslog-2984 Url=tcp+tls://www2.domain2.com:10517
    Updating resource... OK

## rack resources unlink

Unlink a resource from an app

### Usage

    convox rack resources unlink <resource>

### Examples

    $ convox rack resources unlink syslog-2984 -a myapp
    Unlinking from myapp... OK

## rack resources url

Get url for a resource

### Usage

    convox rack resources url <resource>

### Examples

    $ convox rack resources url syslog-2984 -r test
    tcp+tls://www2.domain2.com:10517