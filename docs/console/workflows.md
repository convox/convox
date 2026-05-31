---
title: "Workflows"
description: "Automate building, testing, and deploying applications when code is pushed, using Deployment Workflows for push-to-deploy and Review Workflows for PR previews."
slug: workflows
url: /console/workflows
---
# Workflows

Workflows automate building, testing, and deploying your applications when code changes are pushed to your repository. The Console supports two types: **Deployment Workflows** and **Review Workflows**.

Workflows require a [source integration](/console/integrations) (GitHub or GitLab) to connect your repositories.

Navigate to **CI/CD > Workflows** in the Console sidebar.

## Deployment Workflows

Deployment Workflows build and deploy your application when code is pushed or merged to a specified branch.

### Configuration

Each Deployment Workflow specifies:

| Field | Description |
|-------|-------------|
| **Repository** | The source repository (from your connected integration) |
| **Workflow Name** | A label for this workflow |
| **Branch** | The branch to watch for pushes (e.g. `main`, `production`) |
| **Manifest** | The `convox.yml` file path (defaults to `convox.yml`) |
| **Build Args** | Custom `--build-arg` flags passed to Docker at build time |

### Application Tasks

Each Deployment Workflow contains one or more application tasks. Each task targets an App and supports:

- **App:** The Rack and App to deploy to.
- **Promote:** `Automatic` (deploy immediately after build) or `Manual` (require explicit promotion).
- **Run tests:** Execute the `test:` section from your `convox.yml` after build.
- **Before Promote:** A command to run on a specific Service before promotion (e.g. `bin/migrate`).
- **After Promote:** A command to run on a specific Service after promotion.
- **Rollback if fails:** Automatically roll back the Release if the after-promote command fails.

A single Deployment Workflow can deploy to multiple Apps from one repository push. Each App gets its own promotion strategy and pre/post-deploy hooks.

### Build Args

Expand the **Build Args** section to pass custom arguments to your Dockerfile. Each argument must be declared with `ARG KEY` in your Dockerfile. Enable **Make Convox Managed Build Args Available** to inject Convox-provided variables (App name, Rack, Release ID) automatically.

```text
ARG1=value1
ARG2=value2
```

## Review Workflows

Review Workflows trigger on pull requests, building your application and optionally deploying a temporary preview environment for each PR.

### Configuration

| Field | Description |
|-------|-------------|
| **Repository** | The source repository |
| **Manifest** | The `convox.yml` file path |
| **Branch Pattern** | A regex to filter which PRs trigger this workflow |
| **Branch Type** | Match the pattern against the PR's **source branch** (feature branch) or **target branch** (base branch) |
| **Rack/Machine** | The Rack or Cloud Machine where preview Apps are created |

### Options

- **Deploy demo:** Deploy a temporary App for each PR, automatically removed when the PR is closed.
- **Run tests:** Execute tests and report results to PR status checks.
- **Wildcard Domain:** Generate a unique subdomain for each review App (e.g. `pr-123.yourapp.example.com`).

### Promote Commands

Run commands before or after the Release is promoted:

- **Before Promote:** Service and command to run before promotion (e.g. database migrations).
- **After Promote:** Service and command to run after promotion (e.g. cache clearing).

### Build Args

Same as Deployment Workflows. Pass custom `--build-arg` flags and optionally enable Convox-managed build variables.

### Build Parameters

Override the default Build resource allocation for review Apps:

- **Inherit from app:** Copy Build CPU, memory, and node labels from an existing App.
- **Manual:** Set build CPU (millicores), memory (MB), and node labels directly.

### Environment

Set environment variables injected at deploy time for review Apps. Enable **Environment Override** to re-inject variables on every new commit pushed to the PR.

```text
DATABASE_URL=postgres://review-db:5432/myapp
RAILS_ENV=staging
```

## Workflow Execution

When a Workflow triggers, it creates a **Job** visible under **CI/CD > Jobs**. The Job tracks each step: build, test, promote, and any pre/post-deploy commands. Failed Jobs send notifications to configured [notification integrations](/console/notifications).

## See Also

- [Integrations](/console/integrations)
- [Notifications](/console/notifications)
- [Service](/reference/primitives/app/service)
- [Deployment Workflows](/deployment/workflows)
