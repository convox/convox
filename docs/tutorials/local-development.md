---
title: "Local Development"
slug: local-development
url: /tutorials/local-development
---

# Local Development

This tutorial walks you through installing a development Rack and deploying your first application to it.

## Prerequisites

Before you begin, install the following:

- [Convox CLI](/installation/cli)
- [Development Rack](/installation/development-rack)

## Verify your rack

Confirm that your development Rack is running:

```bash
$ convox rack
Name      dev
Provider  local
Router    router.dev.localdev.convox.cloud
Status    running
Version   3.23.3
```

## Deploy an example application

### Clone the example

```bash
git clone https://github.com/convox-examples/rails.git
cd rails
```

### Review the convox.yml

```yaml
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
```

This manifest defines:

- A `web` [Service](/reference/primitives/app/service) that listens on port 3000 with a health check at `/health`
- A PostgreSQL [Resource](/reference/primitives/app/resource) named `database`, whose connection string is injected as the `DATABASE_URL` environment variable

### Create the app

```bash
convox apps create rails
```

### Start local development

```bash
convox start
```

The first run will take longer than usual as images are pulled and built for the first time.

Once running, you will see application logs streaming in your terminal.

### View the application

In a second terminal, find the URL for your application:

```bash
$ convox services
SERVICE  DOMAIN                                PORTS
web      web.rails.dev.localdev.convox.cloud   443:3000
```

Open `https://web.rails.dev.localdev.convox.cloud` in your browser.

> Your browser will show a certificate warning because the local Rack uses self-signed TLS certificates. This is expected for local development.

### Make a change

Edit `config/routes.rb` and add the following line before the final `end`:

```ruby
get "/test", to: proc { [200, {}, ["Hello World!"]] }
```

You will see the file sync in your first terminal:

```text
convox | sync: config/routes.rb to /usr/src/app on web
```

Open `https://web.rails.dev.localdev.convox.cloud/test` to see your change.

### Run a command

Use `convox run` to execute one-off commands against your application:

```bash
convox run web rake db:migrate
```

## Next steps

- [Deploying an Application](/tutorials/deploying-an-application) to deploy to a production Rack
- [Running Locally](/development/running-locally) for more details on `convox start`
