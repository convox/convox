---
title: "Deploy Keys"
description: "Deploy keys are limited-scope API keys that run a restricted set of convox commands from CI systems without exposing your user credentials."
slug: deploy-keys
url: /management/deploy-keys
---

# Deploy Keys

Deploy keys are limited scope API keys that allow you to run some limited commands from a remote environment (such as continuous integration systems like Jenkins, Travis, CircleCI etc) without needing to store/use/expose your user credentials.

> Create a free Convox account if you don't already have one, sign up [here](https://console.convox.com/signup). We recommend using your company email address if you have one, and using your actual company name as the organization name.

A deploy key with no role assigned is restricted to the commands listed below. Assigning a custom role to the key replaces that restriction entirely, as described in [Custom Roles with Deploy Keys](#custom-roles-with-deploy-keys).

## What a Deploy Key Can Do

Without a role, a deploy key is limited to these commands for security reasons:

| Command | Notes |
|---------|-------|
| `convox apps info` | |
| `convox build` | |
| `convox builds` | |
| `convox builds info` | |
| `convox builds logs` | |
| `convox builds export` | |
| `convox builds import` | |
| `convox deploy` | |
| `convox env set --replace` | The `--replace` is required, see below |
| `convox exec` | |
| `convox logs` | Without `--service`. The per-Service view is not permitted |
| `convox ps info` | |
| `convox ps stop` | |
| `convox rack` | |
| `convox racks` | |
| `convox releases promote` | Only when you name the Release id |
| `convox run` | |
| `convox cloud machines` | |

The Workflow API is also available to a deploy key through the `X-Deploy-Token` header.

### Why env set Needs --replace

Without `--replace`, `convox env set` reads the current environment before writing, and reading the environment means reading a Release. A roleless deploy key cannot read Releases, so the command fails. With `--replace` the CLI writes the new environment directly and never reads one.

The same dependency blocks `convox env`, `convox env get`, `convox env edit` and `convox env unset`.

### Why releases promote Needs an Explicit Id

Called with no id, `convox releases promote` lists Releases to find the latest one, and listing Releases is not permitted. Passing the id skips the lookup.

## What a Deploy Key Cannot Do

The permitted list is closed, so anything not on it is rejected:

```text
ERROR: operation not permitted for deploy keys
```

The cases users hit most often:

| Command | Reason |
|---------|--------|
| `convox releases`, `convox releases info`, `convox releases manifest` | Reads Releases |
| `convox releases rollback`, `convox releases create-from` | Reads a Release first |
| `convox env`, `convox env get`, `convox env edit`, `convox env unset` | Reads the current Release |
| `convox env set` without `--replace` | Reads the current Release |
| `convox logs --service` | The whole-App view is permitted, the per-Service view is not |
| `convox ps` | `convox ps info` for a single Process is permitted, the list is not |
| `convox ssl` | Lists Services and certificates |
| `convox apps cancel` | |
| `convox services`, `convox scale`, `convox restart`, `convox certs`, `convox resources`, `convox cp`, `convox apps create`, `convox apps delete`, `convox rack params set`, `convox rack update` | |

A Release carries the App's full environment, so read access to Releases is read access to every environment variable and secret in the App. Keeping Release reads off the default deploy key scope stops a CI credential from becoming an environment dump. Automation that needs them should use a key with a role scoped to the Apps involved.

## Custom Roles with Deploy Keys

Assigning a [custom role](/management/rbac) to a deploy key **replaces** the command restriction rather than adding to it. The permitted list above stops applying, and the role's policies decide everything the key can do, including the builds and promotes it already runs. Scope the role to cover all of it.

Assign a role when you create the key, or change it later from the **Role** column.

### A Role That Reaches an App Through a Rack Needs Two Policies

Every Rack-proxied call is authorized twice, once against the named resource and once for Read on the Rack the resource lives on. A role carrying only an App policy is denied.

| Resource type | Resources | Action |
|---------------|-----------|--------|
| Rack | the Racks the key needs | Read |
| App | the Apps the key needs | Write |

Two things about how policies match:

- App policies match on the bare App name, not `rack/app`, even though the role editor displays `rack/app`. The Rack policy scopes a key to a subset of Racks.
- Write covers reads, so a separate read policy on the same resource is unnecessary.

### Plan Requirements

Custom roles require a plan that includes them. The role picker offers your organization's custom roles only, not the built-in system roles.

| Plan | Deploy keys | Custom roles |
|------|-------------|--------------|
| Free | 0 | 0 |
| Basic | 2 | 0 |
| Pro | 5 | 20 |
| Plus | 10 | 100 |
| Enterprise | 100000 | 1000 |

Use a dedicated deploy key for each pipeline that needs elevated scope rather than widening the key that runs your ordinary deploys.

## Creating a Deploy Key

Log into your account at [console.convox.com](https://console.convox.com) and open **Deploy Keys** from the left navigation.

Give your deploy key a name, optionally set an expiration and a role, and click **Create**. The table lists each key with its expiration and role. Creating, updating and deleting a key is recorded in the audit log.

> Deploy keys are specific to the organization they are created within. They can only be run against Racks within the same organization.

## Using a Deploy Key

In your CI environment, download the latest version of the [Convox CLI](/getting-started/introduction#install-the-convox-cli-and-log-in) and use the deploy key like these examples:

```sh
$ env CONVOX_HOST=console.convox.com CONVOX_PASSWORD=<key> convox deploy
$ env CONVOX_HOST=console.convox.com CONVOX_PASSWORD=<key> convox run web bin/migrate
$ env CONVOX_HOST=console.convox.com CONVOX_PASSWORD=<key> convox env set NODE_ENV=production FOO=bar ... --replace
$ env CONVOX_HOST=console.convox.com CONVOX_PASSWORD=<key> convox builds export <build ID> -a <app1> -r <rack1> | convox builds import -a <app2> -r <rack2>
```

## See Also

- [RBAC](/management/rbac) for building the custom role a deploy key can be assigned
- [Workflows](/console/workflows) for running builds and deploys from the Console
- [env](/reference/cli/env) for the `--replace` behavior described above
