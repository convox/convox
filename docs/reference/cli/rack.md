# rack

## rack

Get information about the rack

### Usage

    convox rack

### Examples

    $ convox rack
    Name      test
    Provider  aws
    Region    us-east-1
    Router    test-Router-UABCD12E63F45-1234567890.us-east-1.elb.amazonaws.com
    Status    running
    Version   20200116125110-20200116

## rack install

Install a new Rack

### Usage

    convox rack install <provider> <name> [option=value]...

### Examples

    $ convox rack install local dev
    ...

    $ convox rack install aws production region=eu-west-1 node_type=t3.large
    ...

## rack logs

Get logs for the rack

### Usage

    convox rack logs

### Examples

    $ convox rack logs
    2020-02-10T13:37:22Z service/web/a55eb25e-90f5-4301-99fd-e35c91128592 ns=provider.aws at=SystemGet state=success elapsed=275.683
    2020-02-10T13:37:22Z service/web/a55eb25e-90f5-4301-99fd-e35c91128592 id=8d3ec85dc324 ns=api at=SystemGet method="GET" path="/system" response=200 elapsed=276.086
    2020-02-10T13:38:04Z service/web/a55eb25e-90f5-4301-99fd-e35c91128592 ns=provider.aws at=SystemGet state=success elapsed=331.824
    2020-02-10T13:38:04Z service/web/a55eb25e-90f5-4301-99fd-e35c91128592 id=f492a0dce931 ns=api at=SystemGet method="GET" path="/system" response=200 elapsed=332.219
    ...

## rack mv

Transfer the management of a Rack from an individual user to an organization or vice versa.

### Usage

    convox rack mv <from> <to>

### Examples

    $ convox rack mv dev acme/dev
    moving rack dev to acme/dev... OK

    $ convox rack mv acme/dev dev
    moving rack acme/dev to dev... OK

## rack ps

List rack processes

### Usage

    convox rack ps

### Examples

    $ convox rack ps
    ID                       APP     SERVICE        STATUS   RELEASE       STARTED      COMMAND
    api-9749b7ccb-29zh5      system  api            running  3.0.0.beta44  2 weeks ago  api
    api-9749b7ccb-29zh5      rack    api            running  3.0.0.beta44  2 weeks ago  api
    api-9749b7ccb-cg4hr      system  api            running  3.0.0.beta44  2 weeks ago  api
    api-9749b7ccb-cg4hr      rack    api            running  3.0.0.beta44  2 weeks ago  api
    atom-578cd48bfb-6tm7g    rack    atom           running  3.0.0.beta44  2 weeks ago  atom
    atom-578cd48bfb-6tm7g    system  atom           running  3.0.0.beta44  2 weeks ago  atom
    elasticsearch-0          rack    elasticsearch  running  3.0.0.beta44  2 weeks ago
    elasticsearch-0          system  elasticsearch  running  3.0.0.beta44  2 weeks ago
    elasticsearch-1          rack    elasticsearch  running  3.0.0.beta44  2 weeks ago
    elasticsearch-1          system  elasticsearch  running  3.0.0.beta44  2 weeks ago
    fluentd-p56dk            rack    fluentd        running  3.0.0.beta44  2 weeks ago
    fluentd-p56dk            system  fluentd        running  3.0.0.beta44  2 weeks ago
    fluentd-qrttw            rack    fluentd        running  3.0.0.beta44  2 weeks ago
    fluentd-qrttw            system  fluentd        running  3.0.0.beta44  2 weeks ago
    fluentd-zsv8f            rack    fluentd        running  3.0.0.beta44  2 weeks ago
    fluentd-zsv8f            system  fluentd        running  3.0.0.beta44  2 weeks ago
    redis-77b4f65c55-nbx89   rack    redis          running  3.0.0.beta44  2 weeks ago
    redis-77b4f65c55-nbx89   system  redis          running  3.0.0.beta44  2 weeks ago
    router-846b84d544-ndz76  rack    router         running  3.0.0.beta44  2 weeks ago  router
    router-846b84d544-ndz76  system  router         running  3.0.0.beta44  2 weeks ago  router

## rack uninstall

Uninstalls a Rack

### Usage

    convox rack uninstall <name>

### Examples

    $ convox rack uninstall my-rack
    Upgrading modules...
    Downloading github.com/convox/convox?ref=3.0.15 for system...
    ...
    Destroy complete! Resources: 35 destroyed.

## rack update

Updates a Rack to a new version.

### Usage

    convox rack update [version]

### Examples

    $ convox rack update
    Upgrading modules...
    Downloading github.com/convox/convox?ref=3.0.15 for system...
    ...
    Apply complete! Resources: 1 added, 4 changed, 1 destroyed.

    Outputs:

    api = https://convox:password@api.dev.convox
    provider = local
    OK