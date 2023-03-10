---
title: "rack"
draft: false
slug: rack
url: /reference/cli/rack
---
# rack

## rack

Get information about the rack

### Usage
```html
    convox rack
```
### Examples
```html
    $ convox rack
    Name      test
    Provider  aws
    Router    router.0a1b2c3d4e5f.convox.cloud
    Status    running
    Version   3.0.0
```
## rack install

Install a new Rack

### Usage
```html
    convox rack install <provider> <name> [option=value]...
```
### Examples
```html
    $ convox rack install local dev
    ...

    $ convox rack install aws production region=eu-west-1 node_type=t3.large
    ...
```
## rack kubeconfig

Output a Kubernetes configuration file for connecting to the underlying cluster

### Usage
```html
    convox rack kubeconfig
```
### Examples
```html
    $ convox rack kubeconfig
    apiVersion: v1
    clusters:
    - cluster:
        server: https://api.0a1b2c3d4e5f.convox.cloud/kubernetes/
    name: kubernetes
    contexts:
    - context:
        cluster: kubernetes
        user: proxy
    name: proxy@kubernetes
    current-context: proxy@kubernetes
    kind: Config
    preferences: {}
    users:
    - name: proxy
    user:
        username: convox
        password: abcdefghijklmnopqrstuvwxyz
```
## rack logs

Get logs for the rack

### Usage
```html
    convox rack logs
```
### Examples
```html
    $ convox rack logs
    2020-02-10T13:37:22Z service/web/a55eb25e-90f5-4301-99fd-e35c91128592 ns=provider.aws at=SystemGet state=success elapsed=275.683
    2020-02-10T13:37:22Z service/web/a55eb25e-90f5-4301-99fd-e35c91128592 id=8d3ec85dc324 ns=api at=SystemGet method="GET" path="/system" response=200 elapsed=276.086
    2020-02-10T13:38:04Z service/web/a55eb25e-90f5-4301-99fd-e35c91128592 ns=provider.aws at=SystemGet state=success elapsed=331.824
    2020-02-10T13:38:04Z service/web/a55eb25e-90f5-4301-99fd-e35c91128592 id=f492a0dce931 ns=api at=SystemGet method="GET" path="/system" response=200 elapsed=332.219
    ...
```
## rack mv

Transfer the management of a Rack from an individual user to an organization or vice versa.

### Usage
```html
    convox rack mv <from> <to>
```
### Examples
```html
    $ convox rack mv dev acme/dev
    moving rack dev to acme/dev... OK

    $ convox rack mv acme/dev dev
    moving rack acme/dev to dev... OK
```
## rack ps

List rack processes

### Usage
```html
    convox rack ps
```
### Examples
```html
    $ convox rack ps
    ID                       APP     SERVICE        STATUS   RELEASE       STARTED      COMMAND
    api-9749b7ccb-29zh5      system  api            running  3.0.0.beta44  2 weeks ago  api
    api-9749b7ccb-29zh5      rack    api            running  3.0.0.beta44  2 weeks ago  api
    api-9749b7ccb-cg4hr      system  api            running  3.0.0.beta44  2 weeks ago  api
    api-9749b7ccb-cg4hr      rack    api            running  3.0.0.beta44  2 weeks ago  api
    atom-578cd48bfb-6tm7g    rack    atom           running  3.0.0.beta44  2 weeks ago  atom
    atom-578cd48bfb-6tm7g    system  atom           running  3.0.0.beta44  2 weeks ago  atom
    router-846b84d544-ndz76  rack    router         running  3.0.0.beta44  2 weeks ago  router
    router-846b84d544-ndz76  system  router         running  3.0.0.beta44  2 weeks ago  router
```
## rack ps --all

List rack processes as well as essential system ones running on the Rack

### Usage
```html
    convox rack ps --all
```
### Examples
```html
    $ convox rack ps --all
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
```

## rack runtimes

List of attachable runtime integrations

### Usage
```html
    convox rack runtimes
```
### Examples
```html
    $ convox rack runtimes
    ID                                    TITLE
    20e58437-fab7-4124-aa5a-2e5978f1149e  047979207916
```

## rack runtime attach

Attach runtime integration to the rack

### Usage
```html
    convox rack runtime attach <runtime_id>
```
### Examples
```html
    $ convox rack runtime attach 20e58437-fab7-4124-aa5a-2e5978f11
    OK
```

## rack uninstall

Uninstalls a Rack

### Usage
```html
    convox rack uninstall <name>
```
### Examples
```html
    $ convox rack uninstall my-rack
    Upgrading modules...
    Downloading github.com/convox/convox?ref=3.0.15 for system...
    ...
    Destroy complete! Resources: 35 destroyed.
```
## rack update

Updates a Rack to a new version.

### Usage
```html
    convox rack update [version]
```
### Examples
```html
    $ convox rack update
    Upgrading modules...
    Downloading github.com/convox/convox?ref=3.0.15 for system...
    ...
    Apply complete! Resources: 1 added, 4 changed, 1 destroyed.

    Outputs:

    api = https://convox:password@api.dev.convox
    provider = local
    OK
```