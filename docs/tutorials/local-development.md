---
order: 1
---

# Local Development

This tutorial will take you through the process of installing a development Rack on your local workstation
and running your first App on it.

## Installation

Before we begin you will need to install the `convox` CLI and a development Rack:

* [Command Line Interface](../installation/cli.md)
* [Development Rack](../installation/development-rack)

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

## Run the application locally

You can start an app against your development Rack using `convox start`:

> The first time you run `convox start` on an application will take longer than usual
> because you won't have anything cached.

    $ convox start
    build  | uploading source
    build  | starting build
    build  | Authenticating registry.dev.convox/rails: Login Succeeded
    build  | Building: .
    build  | Sending build context to Docker daemon  77.69MB
    build  | Step 1/12 : FROM ruby:2.6.4 AS development
    build  | 2.6.4: Pulling from library/ruby
    build  | Digest: sha256:403a98dc3cc6737cc81ff5143acb50256ce46a65b462c8c75e1942a1c8480852
    build  | Status: Downloaded newer image for ruby:2.6.4
    build  |  ---> 121862ceb25f
    build  | Step 2/12 : ENV RAILS_ENV development
    build  |  ---> Running in 354878b13b32
    build  | Removing intermediate container 354878b13b32
    build  |  ---> 7359d7e18815
    build  | Step 3/12 : RUN apt-get update && apt-get install -y nodejs npm postgresql-client
    build  |  ---> Running in c8540dfe21b6
    build  | Removing intermediate container c8540dfe21b6
    build  |  ---> b3d0acfb372a
    build  | Step 4/12 : RUN npm install -g yarn
    build  |  ---> Running in 8bcd03f9c571
    build  | added 1 package in 0.654s
    build  | Removing intermediate container 8bcd03f9c571
    build  |  ---> 9db86c97841f
    build  | Step 5/12 : WORKDIR /usr/src/app
    build  |  ---> Running in 00e392fb9ffb
    build  | Removing intermediate container 00e392fb9ffb
    build  |  ---> 0ef9e064dee2
    build  | Step 6/12 : COPY Gemfile Gemfile.lock ./
    build  |  ---> e61bf31b9497
    build  | Step 7/12 : RUN bundle install
    build  |  ---> Running in ef927a1ba0d7
    build  | Bundle complete! 17 Gemfile dependencies, 75 gems now installed.
    build  | Bundled gems are installed into `/usr/local/bundle`
    build  | Removing intermediate container ef927a1ba0d7
    build  |  ---> 9ef79cb583f9
    build  | Step 8/12 : COPY package.json yarn.lock ./
    build  |  ---> 5b2b243d810f
    build  | Step 9/12 : RUN yarn install --check-files
    build  |  ---> Running in 861e030377e0
    build  | yarn install v1.22.0
    build  | [1/4] Resolving packages...
    build  | [2/4] Fetching packages...
    build  | [3/4] Linking dependencies...
    build  | [4/4] Building fresh packages...
    build  | Done in 23.49s.
    build  | Removing intermediate container 861e030377e0
    build  |  ---> 63a446385c38
    build  | Step 10/12 : COPY . .
    build  |  ---> 11d5aa716a19
    build  | Step 11/12 : EXPOSE 3000
    build  |  ---> Running in 81e03ee67a6a
    build  | Removing intermediate container 81e03ee67a6a
    build  |  ---> 0153050d030c
    build  | Step 12/12 : CMD ["rails", "server", "-b", "0.0.0.0", "-p", "3000"]
    build  |  ---> Running in b013715c9e03
    build  | Removing intermediate container b013715c9e03
    build  |  ---> b80559b89574
    build  | Successfully built b80559b89574
    build  | Successfully tagged e44305a1288f25f77d83bc4ef6ecf16c974c4b86:latest
    build  | Running: docker tag e44305a1288f25f77d83bc4ef6ecf16c974c4b86 dev/rails:web.BLBXTCIKDBL
    build  | Running: docker tag dev/rails:web.BLBXTCIKDBL registry.dev.convox/rails:web.BLBXTCIKDBL
    build  | Running: docker push registry.dev.convox/rails:web.BLBXTCIKDBL
    convox | starting sync from Gemfile to /usr/src/app/Gemfile.lock on web
    convox | starting sync from . to /usr/src/app on web
    convox | starting sync from package.json to /usr/src/app/yarn.lock on web
    web    | Scaled up replica set web-85bcd457c4 to 1
    web    | Created pod: web-85bcd457c4-m9mkq
    web    | Successfully assigned dev-rails/web-85bcd457c4-m9mkq to docker-desktop
    web    | => Booting Puma
    web    | => Rails 6.0.0 application starting in development
    web    | => Run `rails server --help` for more startup options
    web    | Container image "registry.dev.convox/rails:web.BLBXTCIKDBL" already present on machine
    web    | Created container main
    web    | Started container main
    web    | Puma starting in single mode...
    web    | * Version 3.12.2 (ruby 2.6.4-p104), codename: Llamas in Pajamas
    web    | * Min threads: 5, max threads: 5
    web    | * Environment: development
    web    | * Listening on tcp://0.0.0.0:3000
    web    | Use Ctrl-C to stop
    web    | Started GET "/health" for 10.1.0.1 at 2020-02-11 15:24:11 +0000
    web    | Cannot render console from 10.1.0.1! Allowed networks: 127.0.0.0/127.255.255.255, ::1

## View the application in a browser

In another terminal navigate to the directory where you cloned the example app and run `convox services`:

    $ convox services
    SERVICE  DOMAIN                PORTS
    web      web.rails.dev.convox  443:3000

Now in your browser navigate to https://web.rails.dev.convox

> If you named your development Rack something other than `dev` this URL will be slightly different for you.

You should see logs in the first terminal that show your browser requesting the index page.

    web    | Processing by Rails::WelcomeController#index as HTML
    web    |   Rendering /usr/local/bundle/gems/railties-6.0.0/lib/rails/templates/rails/welcome/index.html.erb
    web    |   Rendered /usr/local/bundle/gems/railties-6.0.0/lib/rails/templates/rails/welcome/index.html.erb (Duration: 3.8ms | Allocations: 194)
    web    | Completed 200 OK in 8ms (Views: 4.4ms | ActiveRecord: 0.0ms | Allocations: 1053)

## Make a change

Edit the file `convox/routes.rb` and add the following line just before the final `end`:

    get "/test", to: proc { [ 200, {}, ["Hello World!"] ] }

This will cause Rails to respond to `GET /test` requests with `Hello World!`.

You will see the change being synchronized up to your development Rack:

    convox | sync: config/routes.rb to /usr/src/app on web

Now navigate to https://web.rails.dev.convox/test

You can make further changes to this route and it will be synchronized every time you save the file.

## Run a command

You can use `convox run` to run [one-off commands](../management/run.md) against the App:

    $ convox run web rake db:migrate test
    Finished in 5.053022s, 0.0000 runs/s, 0.0000 assertions/s.
    0 runs, 0 assertions, 0 failures, 0 errors, 0 skips

## Deploy the application

See the [Deploying an Application](deploying-an-application.md) guide to deploy this application
to the internet.