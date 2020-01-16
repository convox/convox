# apps

## apps

List apps

### Usage

    convox apps

### Examples

    $ convox apps
    APP          STATUS   RELEASE
    myapp        running  RABCDEFGHI
    myapp2       running  RIHGFEDCBA

## apps cancel

Cancel an app update

### Usage

    convox apps cancel [app]

### Examples

    $ convox apps cancel
    Cancelling deployment of myapp... OK

## apps create

Create an app

### Usage

    convox apps create [app]

### Examples

    $ convox apps create myapp
    Creating myapp... OK

## apps delete

Delete an app

### Usage

    convox apps delete <app>

### Examples

    $ convox apps delete myapp

## apps export

Export an app

### Usage

    convox apps export [app]

### Examples

    $ convox apps export --file myapp.tgz
    Exporting app myapp... OK
    Exporting env... OK
    Exporting build BABCDEFGHI... OK
    Exporting resource database... OK
    Packaging export... OK

## apps import

Import an app

### Usage

    convox apps import [app]

### Examples

    $ convox apps import myapp2 --file myapp.tgz
    Creating app myapp2... OK
    Importing build... OK, RIHGFEDCBA
    Importing env... OK, RJIHGFEDCB
    Promoting RJIHGFEDCB... OK
    Importing resource database... OK   

## apps info

Get information about an app

### Usage

    convox apps info [app]

### Examples

    $ convox apps info
    Name        myapp
    Status      running
    Generation  2
    Locked      false
    Release     RABCDEFGHI

## apps lock

Enable termination protection

### Usage

    convox apps lock [app]

### Examples

    $ convox apps lock
    Locking myapp... OK

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
    2020-01-16T14:51:50Z system/cloudformation aws/cfm test-nodejs UPDATE_IN_PROGRESS test-myapp User Initiated
    2020-01-16T14:51:54Z system/cloudformation aws/cfm test-nodejs UPDATE_IN_PROGRESS LogGroup
    2020-01-16T14:51:54Z system/cloudformation aws/cfm test-nodejs UPDATE_IN_PROGRESS ResourceDatabase
    2020-01-16T14:51:55Z system/cloudformation aws/cfm test-nodejs UPDATE_COMPLETE LogGroup
    2020-01-16T14:51:55Z system/cloudformation aws/cfm test-nodejs UPDATE_COMPLETE ResourceDatabase
    2020-01-16T14:51:58Z system/cloudformation aws/cfm test-nodejs UPDATE_IN_PROGRESS ServiceWeb
    2020-01-16T14:51:59Z system/cloudformation aws/cfm test-nodejs UPDATE_COMPLETE ServiceWeb
    2020-01-16T14:52:01Z system/cloudformation aws/cfm test-nodejs UPDATE_COMPLETE_CLEANUP_IN_PROGRESS test-myapp
    2020-01-16T14:52:04Z system/cloudformation aws/cfm test-nodejs UPDATE_IN_PROGRESS ServiceWeb
    2020-01-16T14:52:04Z system/cloudformation aws/cfm test-nodejs UPDATE_COMPLETE ServiceWeb
    2020-01-16T14:52:05Z system/cloudformation aws/cfm test-nodejs UPDATE_IN_PROGRESS ResourceDatabase
    2020-01-16T14:52:05Z system/cloudformation aws/cfm test-nodejs UPDATE_COMPLETE ResourceDatabase
    2020-01-16T14:52:06Z system/cloudformation aws/cfm test-nodejs UPDATE_COMPLETE test-myapp
    OK


## apps unlock

Disable termination protection

### Usage

    convox apps unlock [app]

### Examples

    $ convox apps unlock
    Unlocking myapp... OK
