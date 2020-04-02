# Rails

Our example Rails app can be found [here](https://github.com/convox-examples/rails).  You can clone this locally to run and experiment with.

### Running Locally

Before we begin you will need to install the `convox` CLI and a development Rack:

* [Command Line Interface](../installation/cli.md)
* [Development Rack](../installation/development-rack)

Once you are all setup you can switch to your local rack with ```convox switch [rack name]``` and start your local application with ```convox start``` (make sure you are in the root directory).

You should now be able to access your application by going to [https://web.rails.convox](https://web.rails.convox). If you renamed anything you may need to modify your local URL. The format is https://[service name].[app name].convox

### Custom Application Components

#### [Dockerfile](https://github.com/convox-examples/rails/blob/master/Dockerfile)

Starting from the [ruby:2.6.4](https://hub.docker.com/_/ruby/) image, the `Dockerfile` defines the steps necessary to turn the application code into an image that is ready to run. 

This `Dockerfile` has 3 steps and they are executed in a particular order to take advantage of Docker's build caching behavior.

1. `bundle install` and `yarn install` are run to install dependencies after copying just the files needed to run these commands. This will ensure that the output these commands are cached unless one of these files changes.

2. The application source is copied over. These files will change frequently so this step of the build will very rarely be cached.

3. Finally, after setting the appropriate environment variables the assets are precompiled.

#### [convox.yml](https://github.com/convox-examples/rails/blob/master/convox.yml)

The `convox.yml` manifest explains how to run the application. The manifest for this application has two sections:

1. Resources: These are network-attached dependencies of the application. In this application we have a single resource, a `postgres` database.

2. Services: These are the web-facing services of the application. This application has a single service named `web` which is built from the local directory.

Because the resource named `database` appears in the `links:` section of this service it will receive an environment variable named `DATABASE_URL` with connection details.

#### [config/database.yml](https://github.com/convox-examples/rails/blob/master/config/database.yml)

This file is configured to read database credentials from the `DATABASE_URL` environment variable.

### Deploying to production

Install a production Rack on the cloud provider of your choice:

* [Production Rack](../installation/production-rack)

Once you are all set here you can see the name of your production rack

```bash
convox racks
```

And switch your CLI to your production rack

```bash
convox switch [rack name]
```

Now you can create an empty application in your production rack

```bash
convox apps create
```

And you can deploy your application to production

```bash
convox deploy
```

Finally you can retrieve the URL from your production application with

```bash
convox services
```
