# apps

## apps

List apps

### Usage

    convox apps

### Examples

    $ convox apps
    APP          STATUS   RELEASE
    console      running  RMHPPYFOMID
    datadog      running  RHNIJALRNTB
    django       running
    docs         running  ROCHMCOUESG
    dummy        running  RBINGDLMQJS
    dummy-1      running
    nodejs       running  RCRLBREFPBX
    rails        running
    testenvvars  running  RHXSPWHDFLH
    timer        running  RBNDYOXPUMN
    travis-ci    running

## apps cancel

Cancel an app update

### Usage

    convox apps cancel [app]

### Examples

    $ convox apps cancel
    Cancelling deployment of mynewapp... OK

## apps create

Create an app

### Usage

    convox apps create [app]

### Examples

    $ convox apps create mynewapp
    Creating mynewapp... OK

## apps delete

Delete an app

### Usage

    convox apps delete <app>

### Examples

    $ convox apps delete mynewapp

## apps export

Export an app

### Usage

    convox apps export [app]

### Examples

    $ convox apps export --file mynewapp.tgz
    Exporting app mynewapp... OK
    Exporting env... OK
    Exporting build BPNDBAJXEGW... OK
    Exporting resource database... OK

## apps import

Import an app

### Usage

    convox apps import [app]

### Examples

    $ convox apps import --file mynewapp.tgz
    Importing app mynewapp... OK
    Importing env... OK
    Importing build BPNDBAJXEGW... OK
    Importing resource database... OK    

## apps info

Get information about an app

### Usage

    convox apps info [app]

### Examples

    $ convox apps info
    Name        mynewapp
    Status      running
    Generation  2
    Locked      false
    Release     RCRLBREFPBX

## apps lock

Enable termination protection

### Usage

    convox apps lock [app]

### Examples

    $ convox apps lock
    Locking mynewapp... OK

## apps params

Display app parameters

### Usage

    convox apps params [app]

### Examples

    $ convox apps params
    FargateServices   No
    FargateTimers     No
    IamPolicy
    InternalDomains   Yes
    Isolate           No
    LogBucket         test-logs-jdv3wc1x2s4u
    LogRetention
    Private           No
    Rack              test
    RackUrl           No
    RedirectHttps     Yes
    ResourcePassword  ****
    TaskTags          No
    WebFormation      3,256,512

## apps params set

Set app parameters

### Usage

    convox apps params set <Key=Value> [Key=Value]...

### Examples

    $ convox apps params set LogRetention=3
    Updating parameters...
    2020-01-16T14:51:50Z system/cloudformation aws/cfm test-nodejs UPDATE_IN_PROGRESS test-mynewapp User Initiated
    2020-01-16T14:51:54Z system/cloudformation aws/cfm test-nodejs UPDATE_IN_PROGRESS LogGroup
    2020-01-16T14:51:54Z system/cloudformation aws/cfm test-nodejs UPDATE_IN_PROGRESS ResourceDatabase
    2020-01-16T14:51:55Z system/cloudformation aws/cfm test-nodejs UPDATE_COMPLETE LogGroup
    2020-01-16T14:51:55Z system/cloudformation aws/cfm test-nodejs UPDATE_COMPLETE ResourceDatabase
    2020-01-16T14:51:58Z system/cloudformation aws/cfm test-nodejs UPDATE_IN_PROGRESS ServiceWeb
    2020-01-16T14:51:59Z system/cloudformation aws/cfm test-nodejs UPDATE_COMPLETE ServiceWeb
    2020-01-16T14:52:01Z system/cloudformation aws/cfm test-nodejs UPDATE_COMPLETE_CLEANUP_IN_PROGRESS test-mynewapp
    2020-01-16T14:52:04Z system/cloudformation aws/cfm test-nodejs UPDATE_IN_PROGRESS ServiceWeb
    2020-01-16T14:52:04Z system/cloudformation aws/cfm test-nodejs UPDATE_COMPLETE ServiceWeb
    2020-01-16T14:52:05Z system/cloudformation aws/cfm test-nodejs UPDATE_IN_PROGRESS ResourceDatabase
    2020-01-16T14:52:05Z system/cloudformation aws/cfm test-nodejs UPDATE_COMPLETE ResourceDatabase
    2020-01-16T14:52:06Z system/cloudformation aws/cfm test-nodejs UPDATE_COMPLETE test-mynewapp
    OK


## apps unlock

Disable termination protection

### Usage

    convox apps unlock [app]

### Examples

    $ convox apps unlock
    Unlocking mynewapp... OK
