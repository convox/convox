---
title: "Introduction"
draft: false
slug: Introduction
url: /getting-started/introduction
---

# Introduction

Convox is an open-source Platform as a Service that you can install on your own infrastructure.

## First Steps

We recommend you follow these tutorials in this order:

* [Create a free Convox account](#create-a-free-convox-account)
* [Install the Convox CLI and Login](#install-the-convox-cli-and-login)
* [Install a Runtime Integration for the Cloud of Your Choice](#install-a-runtime-integration-for-the-cloud-of-your-choice)
* [Install a Rack](#install-a-rack)
* [Deploy a Sample App](#deploy-a-sample-app)
* [Make a Code Change and Redeploy](#make-a-code-change-and-redeploy)
* [Perform a Rollback](#perform-a-rollback)

## Create a free Convox account

To create a Convox account simply signup [here](https://console.convox.com/signup). We recommend using your company email address if you have one, and using your actual company name as the organization name. You can invite your colleagues to the organization later. 

## Install the Convox CLI and Login

To install the Convox CLI follow the instructions for your operating system:

### macOS
```html
    $ curl -L https://github.com/convox/convox/releases/latest/download/convox-macos -o /tmp/convox
    $ sudo mv /tmp/convox /usr/local/bin/convox
    $ sudo chmod 755 /usr/local/bin/convox
```
### Linux
```html
    $ curl -L https://github.com/convox/convox/releases/latest/download/convox-linux -o /tmp/convox
    $ sudo mv /tmp/convox /usr/local/bin/convox
    $ sudo chmod 755 /usr/local/bin/convox
```
Once you have installed the CLI you can login to your Convox account by copying the login command from the web console

![CLI Login](/images/documentation/getting-started/introduction/CLI_tutorial_login.png)

If you ever need to login into the CLI again you can generate a new CLI key by going to the [account page](https://console.convox.com/account) and clicking on `Reset CLI Key` which will generate a new key and login command.

Alternatively, if you have access to the CLI key and do not wish to reset it, you can run the following and enter your CLI key on the prompt.

$ convox login console.convox.com

## Install a Runtime Integration for the Cloud of Your Choice

Convox currently supports AWS, Google Cloud, Digital Ocean, and Microsoft Azure. In order to install a Convox Rack, you will need a runtime integration. If you click on the integrations link in the web console and click on the plus sign in the runtime section you can select your cloud and create an integration. All integrations use a specific security role that is created just for Convox and can be removed at any time. To create this role you will need an account for your cloud provider that has sufficient permissions which is typically some equivalent of an administrator role.

## Install a Rack

Each time you install Convox you end up with a new [Rack](/reference/primitives/rack).

A Rack is an isolated set of computing resources, network infrastructure, and storage that can contain
one or more [Apps](/reference/primitives/app).

You can use multiple Racks to isolate different environments, different customers, or different business units.

Many users have two Racks, one for staging and one for production.

You can also run a Rack on your local development workstation to develop your app in an environment nearly
identical to production.

Once you have runtime integration setup it's time to create your first Rack! Click on the Racks link in the web console and then click the cloud Install button. You can give your Rack a name like `dev` and select the region you want to install it in. From here you can select the runtime integration you just created and install a Rack. Rack installation typically takes between 5-20 minutes depending on the cloud provider. If you click on the Rack as it is been installed you can follow along with the Rack creation progress.

![Rack Install](/images/documentation/getting-started/introduction/runtime_tutorial.png)

## Deploy a Sample App

One of the easiest ways to get familiar with Convox is to clone one of our sample apps. For this tutorial, we will use a simple [Node.js app](https://github.com/convox-examples/nodejs) that you can clone from our [example repository](https://github.com/convox-examples)

### Clone the Example App
```html
    $ git clone https://github.com/convox-examples/nodejs.git
```
### Enter the directory with the sample application
```html
    $ cd nodejs
```
### Take a look at the convox.yml
```html
    $ cat convox.yml
    environment:
      - PORT=3000
    services:
      web:
        build: .
        port: 3000
```
This `convox.yml` defines a global [Environment Variable](/configuration/environment) named `port` and one [Service](/reference/primitives/app/service) named `web`. Each
[Process](/reference/primitives/app/process) of this Service will listen on the specified port. This app is a very simple example but the options availble for [`convox.yml`](/configuration/convox-yml) allow you to specify very complex apps made up of many [Services](/reference/primitives/app/service) and [Resources](/reference/primitives/app/resource)

### Create an App in Your Rack

Before you can deploy for the first time you need to create an empty [App](/reference/primitives/app) to deploy to.

First, we can verify that we are logged into our organization and are able to connect to our Rack by listing the current Racks with the `convox racks` command
```html
    $ convox racks
    NAME               PROVIDER  STATUS
    org/dev            aws       running
```
Then we can connect to the Rack we just created with `convox switch`
```html
    $ convox switch dev
```
Now we can create an empty app in our Rack with `convox apps create`
```html
    $ convox apps create
```
By default, the app will be created using the name of the current directory. If you wish to call your app something else you can specify the name as an argument
```html
    $ convox apps create [name]
```
> CLI commands that are specific to an app either take an `-a appname` option or can infer the app
> name from the name of the local directory; in this case `nodejs`

### Deploy Your App
```html
    $ convox deploy
    Packaging source... OK
    Uploading source... OK
    Starting build... OK
    Authenticating 782231114432.dkr.ecr.us-east-1.amazonaws.com: Login Succeeded
    Building: .
    Sending build context to Docker daemon  48.95MB
    Step 1/5 : FROM node:10.16.3-alpine
    ..........
```
> The very first time you deploy a new app into a Rack can take a few minutes depending on the size of the app but subsequent deploys will be much faster

Once the deployment has completed you can verify the app is running by running the `convox services` command in the terminal to get the URL(s) for all running services:
```html
    $ convox services
    SERVICE  DOMAIN                               PORTS
    web      web.nodejs.0a1b2c3d4e5f.convox.cloud  443:3000
```
In your browser navigate to the hostname shown for the `web` service.

### Make a Code Change and Redeploy

Edit the file [`app.js`](https://github.com/convox-examples/nodejs/blob/master/app.js) in your local directory and modify the line that says:

`res.end('Hello Convox!\nI\'m: ' ...`

to instead say:

`res.end('Hello World!\nI\'m: ' ...`

and then save the file. 

Now, you can re-deploy the app by once again running the deploy command.
```html
    $ convox deploy
```
> You should notice that this deploy is much faster.

Once the deployment is complete wait a few seconds and then reload your browser to see the new message.

### Perform a Rollback

One of the great features of Convox is nearly instantaneous rollbacks. Convox performs a rollback by maintaining the history and state of all previous [`Releases`](/reference/primitives/app/release). You can view a list of all the releases for your app with the `convox releases command`
```html
    $ convox releases
    ID           STATUS  BUILD        CREATED        DESCRIPTION
    RSBSSIPZIEF  active  BUTQJQRIWUZ  2 minutes ago  
    RYCWZIXLKQT          BVXIJQDCELS  30 minutes ago     
```
You can now rollback to your previous release with `convox releases rollback`
```html
    $ convox releases rollback RYCWZIXLKQT
    Rolling back to RYCWZIXLKQT... OK, RSPAOICEBER
    Promoting RSPAOICEBER... 
    ...
```
Once the rollback is complete, refresh your browser and you should see the `Hello World!` message has been reverted to `Hello Convox!`

Hopefully, this small example has given you an idea of how easy and powerful Convox is. As a next step let's get your first custom app configured and deployed by following the [App Configuration Guide](/tutorials/preparing-an-application)


