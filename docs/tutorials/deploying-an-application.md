---
order: 2
---

# Deploying an Application

This tutorial will take you through the process of deploying an App to a production Rack.

## Installation

Before we begin you will need to install a production Rack:

We recommend following the [getting started guide](/getting-started/introduction) and installing a Rack via the web console but you can optionally install a Rack using the [command line Rack install instructions](../installation/production-rack)

## Verify installation

Verify that your production Rack is running with `convox rack`:

    $ convox rack
    Name     production
    Provider do
    Router   router.production.convox
    Status   running
    Version  3.0.18

## Get an example applicaton

### Clone the NodeJS example

    $ git clone https://github.com/convox-examples/nodejs.git

### Enter the directory with the example application

    $ cd nodejs

### Look at the convox.yml

    $ cat convox.yml
  
    environment:
        - PORT=3000
    services:
        web:
            build: .
            port: 3000

This `convox.yml` defines one [Service](../reference/primitives/app/service.md) named `web` that will be built from the [Dockerfile](https://github.com/convox-examples/nodejs/blob/master/Dockerfile) in the repo. Each
[Process](../reference/primitives/app/process.md) of this Service will listen on port `3000`.

## Deploy the application

First you will need to create an App:

    $ convox apps create nodejs

Once this completes, you can deploy the code:

    $ convox deploy
    Packaging source... OK
    Uploading source... OK
    Starting build... OK
    Authenticating registry.05456db021737ab6.convox.cloud/nodejs: Login Succeeded
    Building: .
    Sending build context to Docker daemon  48.96MB
    Step 1/5 : FROM node:10.16.3-alpine
    10.16.3-alpine: Pulling from library/node
    e7c96db7181b: Pulling fs layer
    50958466d97a: Pulling fs layer
    56174ae7ed1d: Pulling fs layer
    284842a36c0d: Pulling fs layer
    284842a36c0d: Waiting
    e7c96db7181b: Verifying Checksum
    e7c96db7181b: Download complete
    56174ae7ed1d: Verifying Checksum
    56174ae7ed1d: Download complete
    50958466d97a: Verifying Checksum
    50958466d97a: Download complete
    e7c96db7181b: Pull complete
    284842a36c0d: Verifying Checksum
    284842a36c0d: Download complete
    50958466d97a: Pull complete
    56174ae7ed1d: Pull complete
    284842a36c0d: Pull complete
    Digest: sha256:77c898d0da5e7bfb6e05c9a64de136ba4e03889a72f3c298e95df822a38f450d
    Status: Downloaded newer image for node:10.16.3-alpine
    ---> b95baba1cfdb
    Step 2/5 : WORKDIR /usr/src/app
    ---> Running in 19306c264a03
    Removing intermediate container 19306c264a03
    ---> 83aac0816456
    Step 3/5 : COPY . /usr/src/app
    ---> 6c88d15e88c9
    Step 4/5 : EXPOSE 3000
    ---> Running in b99318863aa8
    Removing intermediate container b99318863aa8
    ---> db735909e450
    Step 5/5 : CMD ["node", "app.js"]
    ---> Running in f1df464405be
    Removing intermediate container f1df464405be
    ---> 7bd8ca94d031
    Successfully built 7bd8ca94d031
    Successfully tagged 81719b4e5da8a821215126cc7edefe7fc2e5751f42db0d920ddbf574:latest
    Running: docker tag 81719b4e5da8a821215126cc7edefe7fc2e5751f42db0d920ddbf574 do-test/nodejs:web.BNLGVRXMCAP
    Running: docker tag do-test/nodejs:web.BNLGVRXMCAP registry.05456db021737ab6.convox.cloud/nodejs:web.BNLGVRXMCAP
    Running: docker push registry.05456db021737ab6.convox.cloud/nodejs:web.BNLGVRXMCAP

    Promoting RHQFJKRJNHJ... 
    2020-05-14T21:37:13Z system/k8s/atom/app Status: Running => Pending
    2020-05-14T21:37:14Z system/k8s/web-749dd486d8 Created pod: web-749dd486d8-8v4ss
    2020-05-14T21:37:14Z system/k8s/web-749dd486d8-8v4ss Successfully assigned do-test-nodejs/web-749dd486d8-8v4ss to do-test-node-3n6m6
    2020-05-14T21:37:14Z system/k8s/web Scaled up replica set web-749dd486d8 to 1
    2020-05-14T21:37:15Z system/k8s/atom/app Status: Pending => Updating
    2020-05-14T21:37:16Z system/k8s/web-749dd486d8-8v4ss Pulling image "registry.05456db021737ab6.convox.cloud/nodejs:web.BNLGVRXMCAP"
    2020-05-14T21:37:22Z system/k8s/web-749dd486d8-8v4ss Successfully pulled image "registry.05456db021737ab6.convox.cloud/nodejs:web.BNLGVRXMCAP"
    2020-05-14T21:37:22Z system/k8s/web-749dd486d8-8v4ss Created container main
    2020-05-14T21:37:22Z system/k8s/web-749dd486d8-8v4ss Started container main
    2020-05-14T21:37:27Z system/k8s/atom/app Status: Updating => Running
    OK


> CLI commands that are specific to an app either take an `-a appname` option or can infer the app
> name from the name of the local directory; in this case `nodejs`

## View the application in a browser

You can get the URL for your running services with the `convox services` command:

    $ convox services
    SERVICE  DOMAIN                               PORTS
    web      web.nodejs.0a1b2c3d4e5f.convox.cloud  443:3000

In your browser navigate to the hostname shown for the `web` service. (i.e. `https://web.nodejs.0a1b2c3d4e5f.convox.cloud/`)

## List the processes of the application

    $ convox ps
    ID                    SERVICE  STATUS   RELEASE     STARTED       COMMAND
    web-0123456789-abcde  web      running  RBCDEFGHIJ  1 minute ago

## View the application logs

    $ convox logs
    2020-01-01T00:00:00Z service/web/web-0123456789-abcde Processing by Rails::WelcomeController#index as HTML
    2020-01-01T00:00:00Z service/web/web-0123456789-abcde   Rendering /usr/local/bundle/gems/railties-6.0.0/lib/rails/templates/rails/welcome/index.html.erb
    2020-01-01T00:00:00Z service/web/web-0123456789-abcde   Rendered /usr/local/bundle/gems/railties-6.0.0/lib/rails/templates/rails/welcome/index.html.erb (Duration: 3.8ms | Allocations: 194)
    2020-01-01T00:00:00Z service/web/web-0123456789-abcde Completed 200 OK in 8ms (Views: 4.4ms | ActiveRecord: 0.0ms | Allocations: 1053)

Notice that the prefix of each log line contains the time that it was received along with the name
of the Service and the ID of the Process that produced it.

Use Ctrl-C to stop following the logs. 

## Scale the application

    $ convox scale web --count=2
    Scaling web...
    2020-01-01T00:00:00Z system/k8s/web-0123456789-zwxwv Pulling image "registry.dev.convox/rails:web.BABCDEFGHI"
    2020-01-01T00:00:00Z system/k8s/web-0123456789-zwxwv Successfully pulled image "registry.dev.convox/rails:web.BABCDEFGHI"
    2020-01-01T00:00:00Z system/k8s/web-0123456789-zwxwv Created container main
    2020-01-01T00:00:00Z system/k8s/web-0123456789-zwxwv Started container main
    OK

Now try listing the processes again:

    ID                    SERVICE  STATUS   RELEASE     STARTED        COMMAND
    web-0123456789-abcde  web      running  RBCDEFGHIJ  2 minutes ago
    web-0123456789-zwxwv  web      running  RBCDEFGHIJ  1 minute ago

## Set an environment variable

    $ convox env set TEST=hello
    Setting TEST... OK
    Release: RCDEFGHIJK

List the Releases to see your change:

    $ convox releases
    ID          STATUS  BUILD       CREATED         DESCRIPTION
    RCDEFGHIJK          BABCDEFGHI  1 minute ago    env add:TEST
    RBCDEFGHIJ  active  BABCDEFGHI  10 minutes ago  build 0a1b2c3d4e commit message

## Promote the new release

Promoting a Release starts a rolling deployment:

    $ convox releases promote
    Promoting RQLOXWGZOLK...
    2020-01-01T00:00:00Z system/k8s/atom/app Status: Running => Pending
    2020-01-01T00:00:00Z system/k8s/atom/app Status: Pending => Updating
    2020-01-01T00:00:00Z system/k8s/web Scaled up replica set web-9876543210 to 2
    2020-01-01T00:00:00Z system/k8s/web-9876543210-hijkl Started container main
    2020-01-01T00:00:00Z system/k8s/web-9876542210-qrstu Started container main
    2020-01-01T00:00:00Z system/k8s/web Scaled down replica set web-0123456789 to 0
    2020-01-01T00:00:00Z system/k8s/web-0123456789 Deleted pod: web-0123456789-abcde
    2020-01-01T00:00:00Z system/k8s/web-0123456789 Deleted pod: web-0123456789-zwxwv
    2020-01-01T00:00:00Z system/k8s/atom/app Status: Updating => Running
    OK

> Running `convox releases promote` takes an optional release ID. Running without an
> ID will promote the latest Release.

## Next steps

* Learn more about [deploying changes](../deployment/deploying-changes)
* Learn more about [scaling](../deployment/scaling)
* Create a [review workflow](https://console-docs.convox.com/console/workflows#review-workflows) to automatically create a review app every time you open a pull request
* Create a [deployment workflow](https://console-docs.convox.com/console/workflows#deployment-workflows) to automatically deploy your app every time you merge to master