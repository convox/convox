---
title: "Introduction"
draft: false
slug: Introduction
url: /getting-started/introduction
---

# Introduction

Convox is an open-source Platform as a Service (PaaS) that you can install on your own infrastructure.

## First Steps

Here are some helpful tutorials to get started with Convox:

* [Create a Free Convox Account](#create-a-free-convox-account)
* [Install a Runtime Integration for Your Cloud Provider](#install-a-runtime-integration-for-your-cloud-provider)
* [Install a Rack](#install-a-rack)
* [Install the Convox CLI and Log In](#install-the-convox-cli-and-log-in)
* [Deploy a Sample App](#deploy-a-sample-app)
* [Make a Code Change and Redeploy](#make-a-code-change-and-redeploy)
* [Perform a Rollback](#perform-a-rollback)

## Create a Free Convox Account

To create a Convox account, simply sign up [here](https://console.convox.com/signup). We recommend using your company or organization's email address.

The first time you log in, you will be prompted to create an Organization. An Organization can have multiple users, such as colleagues or project collaborators, who you can invite to join. You can rename your Organization at any time from the Console Settings page.

![Organization Create](/images/documentation/getting-started/introduction/signup_org.png)

## Install a Runtime Integration for Your Cloud Provider

Convox currently supports AWS, Google Cloud, Digital Ocean, and Microsoft Azure. To install a Convox Rack, you need a runtime integration. In the web console, click on the Integrations link, then click the plus sign in the Runtime section to select your cloud provider and create an integration. All integrations use a specific security role created for Convox, which can be removed at any time. Ensure you have an account with sufficient permissions, typically an administrator role, for your cloud provider.

![Runtime Create](/images/documentation/getting-started/introduction/runtime_create.png)

## Install a Rack

Each time you install Convox, you create a new [Rack](/reference/primitives/rack).

A Rack is an isolated set of computing resources, network infrastructure, and storage that can contain one or more [Apps](/reference/primitives/app). You can use multiple Racks to isolate different environments, customers, or business units.

Many users have two Racks: one for staging and one for production. You can also run a Rack on your local development workstation to develop your app in an environment nearly identical to production.

Once you have set up the runtime integration, it's time to create your first Rack! Click on the Racks link in the web console, then click the cloud Install button, select the runtime integration you just created, and install a Rack. Give your Rack a name like `dev` and select the region you want to install it in.

You can further customize your Rack configurations with [Rack Parameters](/configuration/rack-parameters). To start, you can choose one of our predefined Rack Templates or fully customize your Rack to your needs. Unless specified, all Rack Parameters can be changed whenever you'd like, so with a single CLI command, you can change most overall configurations on the fly.

Finally, click install to begin creating your infrastructure. Rack installation typically takes between 5-20 minutes depending on the cloud provider. If you click on the Rack as it is being installed, you can follow along with the Rack creation progress.

![Rack Install](/images/documentation/getting-started/introduction/rack_install.png)


## Install the Convox CLI and Log In

To install the Convox CLI, follow the instructions for your operating system:

### Linux

#### x86_64 / amd64

```bash
$ curl -L https://github.com/convox/convox/releases/latest/download/convox-linux -o /tmp/convox
$ sudo mv /tmp/convox /usr/local/bin/convox
$ sudo chmod 755 /usr/local/bin/convox
```

#### arm64

```bash
$ curl -L https://github.com/convox/convox/releases/latest/download/convox-linux-arm64 -o /tmp/convox
$ sudo mv /tmp/convox /usr/local/bin/convox
$ sudo chmod 755 /usr/local/bin/convox
```

### macOS

#### x86_64 / amd64

```bash
$ curl -L https://github.com/convox/convox/releases/latest/download/convox-macos -o /tmp/convox
$ sudo mv /tmp/convox /usr/local/bin/convox
$ sudo chmod 755 /usr/local/bin/convox
```

#### arm64

```bash
$ curl -L https://github.com/convox/convox/releases/latest/download/convox-macos-arm64 -o /tmp/convox
$ sudo mv /tmp/convox /usr/local/bin/convox
$ sudo chmod 755 /usr/local/bin/convox
```

Once you have installed the CLI, you can log in to your Convox account by copying the login command from the Convox Console checklist/tour page.

![CLI Login Checklist](/images/documentation/getting-started/introduction/cli_checklist.png)

If you ever need to log in to the CLI again, you can generate a new CLI key by going to the [account page](https://console.convox.com/account) and clicking on reset button, which will generate a new key and login command.

![CLI Login Account](/images/documentation/getting-started/introduction/cli_account.png)

Alternatively, if you have access to the CLI key and do not wish to reset it, you can run the following and enter your CLI key on the prompt:

```bash
$ convox login console.convox.com -t 0123456789abcdefghijklmnopqrstuvwxyz
```

For a quick overview of some commonly used CLI commands, check out this [blog post on common Convox CLI commands](https://www.convox.com/blog/common-convox-cli-commands).

## Deploy a Sample App

One of the easiest ways to get familiar with Convox is to clone one of our sample apps. For this tutorial, we will use a simple [Node.js app](https://github.com/convox-examples/nodejs) from our [example repository](https://github.com/convox-examples).

### Clone the Example App

```bash
$ git clone https://github.com/convox-examples/nodejs.git
```

### Enter the Directory with the Sample Application

```bash
$ cd nodejs
```

### Take a Look at the convox.yml

```bash
$ cat convox.yml
environment:
  - PORT=3000
services:
  web:
    build: .
    port: 3000
```

This `convox.yml` defines a global [Environment Variable](/configuration/environment) named `PORT` and one [Service](/reference/primitives/app/service) named `web`. Each [Process](/reference/primitives/app/process) of this Service will listen on the specified port. This app is a simple example, but the options available for [`convox.yml`](/configuration/convox-yml) allow you to specify very complex apps made up of many [Services](/reference/primitives/app/service) and [Resources](/reference/primitives/app/resource).

### Create an App in Your Rack

Before you can deploy for the first time, you need to create an empty [App](/reference/primitives/app) to deploy to.

First, verify that you are logged into your organization and able to connect to your Rack by listing the current Racks with the `convox racks` command:

```bash
$ convox racks
NAME               PROVIDER  STATUS
org/dev            aws       running
```

Then, connect to the Rack you just created with `convox switch`:

```bash
$ convox switch dev
```

Now, create an empty app in your Rack with `convox apps create`:

```bash
$ convox apps create
```

By default, the app will be created using the name of the current directory. If you wish to call your app something else, you can specify the name as an argument:

```bash
$ convox apps create [name]
```

> CLI commands specific to an app either take an `-a appname` option or can infer the app name from the name of the local directory; in this case, `nodejs`.

### Deploy Your App

```bash
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

> The first time you deploy a new app into a Rack can take a few minutes depending on the size of the app, but subsequent deploys will be much faster.

Once the deployment has completed, verify the app is running by running the `convox services` command in the terminal to get the URL(s) for all running services:

```bash
$ convox services
SERVICE  DOMAIN                               PORTS
web      web.nodejs.0a1b2c3d4e5f.convox.cloud  443:3000
```

In your browser, navigate to the hostname shown for the `web` service.

### Make a Code Change and Redeploy

Edit the file [`app.js`](https://github.com/convox-examples/nodejs/blob/master/app.js) in your local directory and modify the line that says:

```javascript
res.end('Hello World!\nI\'m: ' ...
```

to instead say:

```javascript
res.end('Hello Convox!\nI\'m: ' ...
```

Save the file, then re-deploy the app by running the deploy command again:

```bash
$ convox deploy
```

> You should notice that this deploy is much faster.

Once the deployment is complete, wait a few seconds and then reload your browser to see the new message.

### Perform a Rollback

One of the great features of Convox is nearly instantaneous rollbacks. Convox performs a rollback by maintaining the history and state of all previous [Releases](/reference/primitives/app/release). You can view a list of all the releases for your app with the `convox releases` command:

```bash
$ convox releases
ID           STATUS  BUILD        CREATED        DESCRIPTION
RSBSSIPZIEF  active  BUTQJQRIWUZ  2 minutes ago
RYCWZIXLKQT          BVXIJQDCELS  30 minutes ago
```

You can now roll back to your previous release with `convox releases rollback`:

```bash
$ convox releases rollback RYCWZIXLKQT
Rolling back to RYCWZIXLKQT... OK, RSPAOICEBER
Promoting RSPAOICEBER...
...
```

Once the rollback is complete, refresh your browser, and you should see the `Hello Convox!` message has been reverted to `Hello World!`.

Hopefully, this small example has given you an idea of how easy and powerful Convox is. As a next step, let's get your first custom app configured and deployed by following the [App Configuration Guide](/tutorials/preparing-an-application).

## Take a Tour of the Convox Console

To get familiar with the Convox Console and some of its additional features, watch our [Convox - Console Tour](https://www.youtube.com/watch?v=p7f_MzAFxSg&t=25s) video on YouTube. This video provides a comprehensive overview of the console, helping you navigate and utilize its various functionalities effectively.