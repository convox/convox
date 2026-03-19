---
title: "Deploying an Application"
slug: deploying-an-application
url: /tutorials/deploying-an-application
---

# Deploying an Application

This tutorial will take you through the process of deploying an App to a production Rack.

## Installation

Before we begin you will need to install a production Rack:

We recommend following the [getting started guide](/getting-started/introduction) and installing a Rack via the web console but you can optionally install a Rack using the [command line Rack install instructions](/installation/production-rack)

## Verify installation

Verify that your production Rack is running with `convox rack`:
```bash
    $ convox rack
    Name     production
    Provider aws
    Router   router.production.convox
    Status   running
    Version  3.23.3
```
## Get an example application

### Clone the NodeJS example
```bash
    $ git clone https://github.com/convox-examples/nodejs.git
```
### Enter the directory with the example application
```bash
    $ cd nodejs
```
### Look at the convox.yml
```bash
    $ cat convox.yml

    environment:
      - PORT=3000
    services:
      web:
        build: .
        port: 3000
```
This `convox.yml` defines one [Service](/reference/primitives/app/service) named `web` that will be built from the [Dockerfile](https://github.com/convox-examples/nodejs/blob/master/Dockerfile) in the repo. Each
[Process](/reference/primitives/app/process) of this Service will listen on port `3000`.

## Deploy the application

First you will need to create an App:
```bash
    $ convox apps create nodejs
```
Once this completes, you can deploy the code:
```bash
    $ convox deploy
    Packaging source... OK
    Uploading source... OK
    Starting build... OK
    Authenticating 782231114432.dkr.ecr.us-east-1.amazonaws.com: Login Succeeded
    Building: .
    Sending build context to Docker daemon  48.96MB
    Step 1/5 : FROM node:20-alpine
    20-alpine: Pulling from library/node
    e7c96db7181b: Pulling fs layer
    50958466d97a: Pulling fs layer
    56174ae7ed1d: Pulling fs layer
    284842a36c0d: Pulling fs layer
    ...
    Status: Downloaded newer image for node:20-alpine
    ---> b95baba1cfdb
    Step 2/5 : WORKDIR /usr/src/app
    ---> 83aac0816456
    Step 3/5 : COPY . /usr/src/app
    ---> 6c88d15e88c9
    Step 4/5 : EXPOSE 3000
    ---> db735909e450
    Step 5/5 : CMD ["node", "app.js"]
    ---> 7bd8ca94d031
    Successfully built 7bd8ca94d031
    Running: docker push registry.05456db021737ab6.convox.cloud/nodejs:web.BNLGVRXMCAP

    Promoting RHQFJKRJNHJ...
    2026-03-18T14:22:13Z system/k8s/atom/app Status: Running => Pending
    2026-03-18T14:22:14Z system/k8s/web-749dd486d8 Created pod: web-749dd486d8-8v4ss
    2026-03-18T14:22:14Z system/k8s/web-749dd486d8-8v4ss Successfully assigned nodejs/web-749dd486d8-8v4ss to production-node-3n6m6
    2026-03-18T14:22:14Z system/k8s/web Scaled up replica set web-749dd486d8 to 1
    2026-03-18T14:22:15Z system/k8s/atom/app Status: Pending => Updating
    2026-03-18T14:22:16Z system/k8s/web-749dd486d8-8v4ss Pulling image "registry.05456db021737ab6.convox.cloud/nodejs:web.BNLGVRXMCAP"
    2026-03-18T14:22:22Z system/k8s/web-749dd486d8-8v4ss Successfully pulled image "registry.05456db021737ab6.convox.cloud/nodejs:web.BNLGVRXMCAP"
    2026-03-18T14:22:22Z system/k8s/web-749dd486d8-8v4ss Created container main
    2026-03-18T14:22:22Z system/k8s/web-749dd486d8-8v4ss Started container main
    2026-03-18T14:22:27Z system/k8s/atom/app Status: Updating => Running
    OK
```

> CLI commands that are specific to an app either take an `-a appname` option or can infer the app
> name from the name of the local directory; in this case `nodejs`

## View the application in a browser

You can get the URL for your running services with the `convox services` command:
```bash
    $ convox services
    SERVICE  DOMAIN                                PORTS
    web      web.nodejs.0a1b2c3d4e5f.convox.cloud  443:3000
```
In your browser navigate to the hostname shown for the `web` service. (i.e. `https://web.nodejs.0a1b2c3d4e5f.convox.cloud/`)

## List the processes of the application
```bash
    $ convox ps
    ID                    SERVICE  STATUS   RELEASE     STARTED       COMMAND
    web-0123456789-abcde  web      running  RBCDEFGHIJ  1 minute ago
```
## View the application logs
```bash
    $ convox logs
    2026-03-18T14:22:30Z service/web/web-0123456789-abcde Node.js app listening on port 3000
    2026-03-18T14:22:35Z service/web/web-0123456789-abcde GET / 200 3.241 ms
    2026-03-18T14:22:36Z service/web/web-0123456789-abcde GET /favicon.ico 404 1.027 ms
    2026-03-18T14:22:40Z service/web/web-0123456789-abcde GET / 200 0.892 ms
```
Notice that the prefix of each log line contains the time that it was received along with the name
of the Service and the ID of the Process that produced it.

Use Ctrl-C to stop following the logs.

## Scale the application
```bash
    $ convox scale web --count=2
    Scaling web...
    2026-03-18T14:25:00Z system/k8s/web-0123456789-zwxwv Pulling image "registry.05456db021737ab6.convox.cloud/nodejs:web.BNLGVRXMCAP"
    2026-03-18T14:25:03Z system/k8s/web-0123456789-zwxwv Successfully pulled image "registry.05456db021737ab6.convox.cloud/nodejs:web.BNLGVRXMCAP"
    2026-03-18T14:25:03Z system/k8s/web-0123456789-zwxwv Created container main
    2026-03-18T14:25:03Z system/k8s/web-0123456789-zwxwv Started container main
    OK
```
Now try listing the processes again:
```text
    ID                    SERVICE  STATUS   RELEASE     STARTED        COMMAND
    web-0123456789-abcde  web      running  RBCDEFGHIJ  2 minutes ago
    web-0123456789-zwxwv  web      running  RBCDEFGHIJ  1 minute ago
```
## Set an environment variable
```bash
    $ convox env set TEST=hello
    Setting TEST... OK
    Release: RCDEFGHIJK
```
List the Releases to see your change:
```bash
    $ convox releases
    ID          STATUS  BUILD       CREATED         DESCRIPTION
    RCDEFGHIJK          BABCDEFGHI  1 minute ago    env add:TEST
    RBCDEFGHIJ  active  BABCDEFGHI  10 minutes ago  build 0a1b2c3d4e commit message
```
## Promote the new release

Promoting a Release starts a rolling deployment:
```bash
    $ convox releases promote
    Promoting RCDEFGHIJK...
    2026-03-18T14:30:00Z system/k8s/atom/app Status: Running => Pending
    2026-03-18T14:30:01Z system/k8s/atom/app Status: Pending => Updating
    2026-03-18T14:30:02Z system/k8s/web Scaled up replica set web-9876543210 to 2
    2026-03-18T14:30:04Z system/k8s/web-9876543210-hijkl Started container main
    2026-03-18T14:30:05Z system/k8s/web-9876542210-qrstu Started container main
    2026-03-18T14:30:10Z system/k8s/web Scaled down replica set web-0123456789 to 0
    2026-03-18T14:30:10Z system/k8s/web-0123456789 Deleted pod: web-0123456789-abcde
    2026-03-18T14:30:10Z system/k8s/web-0123456789 Deleted pod: web-0123456789-zwxwv
    2026-03-18T14:30:15Z system/k8s/atom/app Status: Updating => Running
    OK
```
> Running `convox releases promote` takes an optional release ID. Running without an
> ID will promote the latest Release.

## Next steps

* Learn more about [deploying changes](/deployment/deploying-changes)
* Learn more about [scaling](/configuration/scaling)
* Create a [review workflow](/deployment/workflows#review-workflows) to automatically create a review app every time you open a pull request
* Create a [deployment workflow](/deployment/workflows#deployment-workflows) to automatically deploy your app every time you merge to master
