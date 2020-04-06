# apps

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

Display app parameters.  (Currently unused for v3 Racks)

### Usage

    convox apps params [app]

### Examples

    $ convox apps params


    $ convox apps params -a my-app -r version-2-rack
    FargateServices   No
    FargateTimers     No
    IamPolicy
    InternalDomains   Yes
    Isolate           No
    LogBucket         my-app-logs-11z00ad8o4933
    LogRetention
    Private           No
    Rack              version-2-rack
    RackUrl           Yes
    RedirectHttps     Yes
    ResourcePassword  ****
    TaskTags          No
    WebFormation      5,64,2000
    WorkerFormation   1,128,500
    

## apps params set

Set app parameters.  (Currently unused for v3 Racks)

### Usage

    convox apps params set <Key=Value> [Key=Value]...

### Examples

    $ convox apps params set LogRetention=14 -a my-app -r version-2-rack
    Setting app parameter... OK

## apps unlock

Disable termination protection

### Usage

    convox apps unlock [app]

### Examples

    $ convox apps unlock
    Unlocking myapp... OK
