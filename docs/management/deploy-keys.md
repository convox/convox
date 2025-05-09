---
title: "Deploy Keys"
draft: false
slug: Deploy Keys
url: /management/deploy-keys
---

# Deploy Keys

Deploy keys are limited scope API keys that allow you to run some limited commands from a remote environment (such as continuous integration systems like Jenkins, Travis, CircleCI etc) without needing to store/use/expose your user credentials.  

> Create a free Convox account if you don't already have one, simply signup [here](https://console.convox.com/signup). We recommend using your company email address if you have one, and using your actual company name as the organization name.

The commands you can run with a deploy key are limited to the following for security reasons:

* `build`
* `builds`
* `builds`
* `builds export`
* `builds import`
* `deploy`
* `env set --replace`
* `logs`
* `rack`
* `racks`
* `releases promote` (only if you specify the release id to promote)
* `run`

## Custom Roles with Deploy Keys

With the [RBAC](/management/rbac) update, you can assign custom roles to Deploy Keys. This allows you to extend the default set of commands a deploy key can execute, providing more granular control over what operations are permitted. By assigning a custom role, you can tailor a deploy key’s permissions to suit your specific needs.

For more details, see the [RBAC documentation](/management/rbac).

## Creating a Deploy Key

To generate a deploy key, log into your account at [console.convox.com](https://console.convox.com) and click on the **Settings** tab on the left.

Go to the **Deploy Keys** section, give your deploy key a name, and click on **Create**.

> Deploy keys are specific to the organization they are created within. They can only be run against Racks within the same organization.

## Using a Deploy Key

In your CI environment, download the latest version of the [Convox CLI](/getting-started/introduction#install-the-convox-cli-and-login) and use the deploy key like these examples:

```sh
$ env CONVOX_HOST=console.convox.com CONVOX_PASSWORD=<key> convox deploy
$ env CONVOX_HOST=console.convox.com CONVOX_PASSWORD=<key> convox run web bin/migrate
$ env CONVOX_HOST=console.convox.com CONVOX_PASSWORD=<key> convox env set NODE_ENV=production FOO=bar ... --replace
$ env CONVOX_HOST=console.convox.com CONVOX_PASSWORD=<key> convox builds export <build ID> -a <app1> -r <rack1> | convox builds import -a <app2> -r <rack2>
```