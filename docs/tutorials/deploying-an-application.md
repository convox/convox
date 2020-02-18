---
order: 2
---

# Deploying an Application

This tutorial will take you through the process of deploying an App to a production Rack.

## Installation

Before we begin you will need to install a production Rack:

* [Production Rack](../installation/production-rack)

## Verify installation

Verify that your development Rack is running with `convox rack`:

    $ convox rack
    Name     dev
    Provider local
    Router   router.dev.convox
    Status   running
    Version  3.0.0

## Get an example applicaton

### Clone the Rails example

    $ git clone https://github.com/convox-examples/rails.git

### Enter the directory with the example application

    $ cd rails

### Look at the convox.yml

    $ cat convox.yml
    resources:
      database:
        type: postgres
    services:
      web:
        build: .
        health: /health
        port: 3000
        resources:
          - database

This `convox.yml` defines one [Service](../reference/primitives/app/service.md) named `web`. Each
[Process](../reference/primitives/app/process.md) of this Service will listen on port `3000` and should
respond with a successful response to a [health check](../configuration/health-checks.md) at `GET /health`.

This App also has one PostgreSQL [Resource](../reference/primitives/app/resource.md) named `database` which
is connected to the `web` [Service](../reference/primitives/app/service.md). This will create a PostgreSQL
database and make its connection information available to the `web` [Processes](../reference/primitives/app/process.md)
as the environment variable `DATABASE_URL`.

## Deploy the application

First you will need to create an App:

    $ convox apps create rails

Once this completes, you can deploy the code:

    $ convox deploy
    uploading source
    starting build
    Authenticating registry.dev.convox/rails: Login Succeeded
    Building: .
    Sending build context to Docker daemon  77.69MB
    Step 1/12 : FROM ruby:2.6.4 AS development
    2.6.4: Pulling from library/ruby
    Digest: sha256:403a98dc3cc6737cc81ff5143acb50256ce46a65b462c8c75e1942a1c8480852
    Status: Downloaded newer image for ruby:2.6.4
     ---> 121862ceb25f
    Step 2/12 : ENV RAILS_ENV development
     ---> Running in 354878b13b32
    Removing intermediate container 354878b13b32
     ---> 7359d7e18815
    Step 3/12 : RUN apt-get update && apt-get install -y nodejs npm postgresql-client
     ---> Running in c8540dfe21b6
    Removing intermediate container c8540dfe21b6
     ---> b3d0acfb372a
    Step 4/12 : RUN npm install -g yarn
     ---> Running in 8bcd03f9c571
    added 1 package in 0.654s
    Removing intermediate container 8bcd03f9c571
     ---> 9db86c97841f
    Step 5/12 : WORKDIR /usr/src/app
     ---> Running in 00e392fb9ffb
    Removing intermediate container 00e392fb9ffb
     ---> 0ef9e064dee2
    Step 6/12 : COPY Gemfile Gemfile.lock ./
     ---> e61bf31b9497
    Step 7/12 : RUN bundle install
     ---> Running in ef927a1ba0d7
    Bundle complete! 17 Gemfile dependencies, 75 gems now installed.
    Bundled gems are installed into `/usr/local/bundle`
    Removing intermediate container ef927a1ba0d7
     ---> 9ef79cb583f9
    Step 8/12 : COPY package.json yarn.lock ./
     ---> 5b2b243d810f
    Step 9/12 : RUN yarn install --check-files
     ---> Running in 861e030377e0
    yarn install v1.22.0
    [1/4] Resolving packages...
    [2/4] Fetching packages...
    [3/4] Linking dependencies...
    [4/4] Building fresh packages...
    Done in 23.49s.
    Removing intermediate container 861e030377e0
     ---> 63a446385c38
    Step 10/12 : COPY . .
     ---> 11d5aa716a19
    Step 11/12 : EXPOSE 3000
     ---> Running in 81e03ee67a6a
    Removing intermediate container 81e03ee67a6a
     ---> 0153050d030c
    Step 12/12 : CMD ["rails", "server", "-b", "0.0.0.0", "-p", "3000"]
     ---> Running in b013715c9e03
    Removing intermediate container b013715c9e03
     ---> b80559b89574
    Successfully built b80559b89574
    Successfully tagged e44305a1288f25f77d83bc4ef6ecf16c974c4b86:latest
    Running: docker tag e44305a1288f25f77d83bc4ef6ecf16c974c4b86 rails:web.BABCDEFGHI
    Running: docker tag rails:web.BABCDEFGHI registry.dev.convox/rails:web.BABCDEFGHI
    Running: docker push registry.dev.convox/rails:web.BABCDEFGHI
    Build: BABCDEFGHI
    Release: RBCDEFGHIJ
    Promoting RKTWNGFBDWU...
    2020-01-01T00:00:00Z system/k8s/atom/app Status: Running => Pending
    2020-01-01T00:00:00Z system/k8s/atom/app Status: Pending => Updating
    2020-01-01T00:00:00Z system/k8s/web Scaled up replica set web-0123456789 to 1
    2020-01-01T00:00:00Z system/k8s/web-5b5dbc69b4 Created pod: web-0123456789-abcde
    2020-01-01T00:00:00Z system/k8s/web-0123456789-abcde Successfully assigned tgcp-httpd/web-0123456789-abcde to gke-tgcp-tgcp-nodes-n1-standard-1-6496634b-64p9
    2020-01-01T00:00:00Z system/k8s/web-0123456789-abcde Pulling image "registry.dev.convox/rails:web.BABCDEFGHI"
    2020-01-01T00:00:00Z system/k8s/web-0123456789-abcde Successfully pulled image "registry.dev.convox/rails:web.BABCDEFGHI"
    2020-01-01T00:00:00Z system/k8s/web-0123456789-abcde Started container main
    2020-01-01T00:00:00Z system/k8s/web-0123456789-abcde Created container main
    2020-01-01T00:00:00Z system/k8s/atom/app Status: Updating => Running
    OK


> CLI commands that are specific to an app either take an `-a appname` option or can infer the app
> name from the name of the local directory; in this case `rails`

## View the application in a browser

In another terminal navigate to the directory where you cloned the example app and run `convox services`:

    $ convox services
    SERVICE  DOMAIN                               PORTS
    web      web.rails.0a1b2c3d4e5f.convox.cloud  443:3000

In your browser navigate the the hostname shown for the `web` service.

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

While this was a simple example we encourage you to take a look at the Configuration
section of the docs to see how you can configure a more complex application.