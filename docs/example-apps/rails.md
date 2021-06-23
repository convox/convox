# Rails

You can find an example Rails app [here](https://github.com/convox-examples/rails).  You can clone this repository locally to run and experiment with it.

### Preparing your environment

Before we begin you will need to install the `convox` CLI and a development Rack:

* [Command Line Interface](../installation/cli.md)
* [Development Rack](../installation/development-rack)

### Preparing your application

To deploy your rails application in your local rack your need to make a few changes to it. 

**1.** Starting from the [ruby:3.0.0](https://hub.docker.com/_/ruby/) image, the `Dockerfile` defines the steps necessary to turn the application code into an image that is ready to run. 

[This](https://github.com/convox-examples/rails/blob/master/Dockerfile) `Dockerfile` has 3 steps and they are executed in a particular order to take advantage of Docker's build caching behavior.

1. `bundle install` and `yarn install` are run to install dependencies after copying just the files needed to run these commands. This will ensure that the output these commands are cached unless one of these files changes.

2. The application source is copied over. These files will change frequently so this step of the build will very rarely be cached.

3. Finally, after setting the appropriate environment variables the assets are precompiled.

To run your application you will need a Dockerfile. 

1. Create a file in the root of your project with the name `Dockerfile` the content of our [example](https://github.com/convox-examples/rails). Notice that ours uses ruby-3.0 as a base image, if you need a different version feel free to change the version on the first line of the file to a valid ruby Docker image.

**2.** The `convox.yml` manifest explains how to run the application. The manifest for this application has two sections:

1. Resources: These are network-attached dependencies of the application. In this application we have a single resource, a `postgres` database.

2. Services: These are the web-facing services of the application. This application has a single service named `web` which is built from the local directory.

Because the resource named `database` appears in the `links:` section of this service it will receive an environment variable named `DATABASE_URL` with connection details.

Create a file in the root of your project with the name `convox.yml` and the following content

Check [convox.yml](https://docs.convox.com/configuration/convox-yml) to see all the possible configurations.

```
resources: # Here we are creating a database resource to use in our application.
  database:
    type: postgres

services:
  web:
    build: .
    port: 3000 # Here we are opening the port in which our applicatin will run. If we need a different port we change it here
    environment: # Here we define the environment variables that we will use in our application. 
      - SECRET_KEY_BASE=
    resources: # Here we are linking our service with the resource we want to use. Since our resource is called "database", Convox will create a DATABASE_URL that can be read by our application.
      - database
```

**3.** To stop Docker from loading unnecessary files you should define a [.dockerignore](https://docs.docker.com/engine/reference/builder/#dockerignore-file) file. Create a file in the root of your project with the name `.dockerignore` and the content of our [example](https://github.com/convox-examples/rails/blob/master/.dockerignore).

**4.** On Rails 6.0+ you need to define any host you will use to access your application. Since Convox generates a host that is different from `localhost` and `0.0.0.0` you need to define it in your application. More information about this configuration [here](https://guides.rubyonrails.org/configuring.html#configuring-middleware).

The URL that Convox automatically generates on your local Rack follows the following format:

```https://[service name].[app name].convox```

In our convox.yml the service name is `web`, thus your URL will be:

```https://web.[app name].convox```

By default, the name of your `app` is taken to be the name of the directory you are in.  You can use a different app name by adding the `--app/-a` flags to any Convox command.

**5.** Assuming you want to use postgres in your application, in your gemfile add the following line `gem 'pg'`. If you want to use a different database you just need to install the gem and define it on the `resources` section of `convox.yml`. You can find information about the databases we currently natively support [here](https://docs.convox.com/reference/primitives/app/resource#types)


**6.** As mentioned in step **#2**, when you define a resource Convox will create an environment variable with that name for your application to access it. For more information on how it works check it [here](https://docs.convox.com/reference/primitives/app/resource#linking). To use Convox' database resource as in our convox.yml, in your `config/database.yml` add the following line under `default`:

 ```  url: <%= ENV['DATABASE_URL'] %>```.
 
Your `database.yml` should look like [this](https://github.com/convox-examples/rails/blob/master/config/database.yml)

### Running Locally

Once you are all setup you can switch to your local rack with ```convox switch dev``` and from your project's folder you can start your local application with ```convox start```.

You should now be able to access your application by going to [https://web.rails.convox](https://web.rails.convox). If you renamed anything you may need to modify your local URL. The format is https://[service name].[app name].convox

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

You need to define the secret key for your production application

```
convox env set SECRET_KEY_BASE="$(rails secret)"
```

And you can deploy your application to production

```bash
convox deploy
```

Finally you can retrieve the URL from your production application with

```bash
convox services
```