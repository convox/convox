---
title: "CI/CD Workflows"
draft: false
slug: CI/CD Workflows
url: /deployment/workflows
---
# Workflows

Workflows enable you to automate continuous integration and continuous delivery processes in your Convox environment. They provide a way to build, test, and deploy your applications automatically in response to events such as code pushes or pull requests.

## Overview

Convox offers two types of workflows:

- **Review Workflows**: Triggered when pull requests are created, allowing you to test and review changes before merging
- **Deployment Workflows**: Triggered when code is merged into specific branches, facilitating continuous deployment to staging and production environments

Workflows integrate directly with GitHub and GitLab, making it easy to automate your development pipeline with minimal configuration.

## Review Workflows

Review workflows allow you to test new versions of your application based on pull requests. When configured, Convox automatically builds your application whenever a pull request is created for a connected repository. Optionally, Convox can run tests and create temporary review applications, enabling thorough testing before merging changes.

### Creating a Review Workflow

1. Navigate to the **Workflows** tab in the Convox Console
2. Click **Create Workflow**
3. Select **Review Workflow**
4. Configure the following settings:

| Setting | Description |
|---------|-------------|
| **Repository** | The connected GitHub or GitLab repository that will trigger the workflow |
| **Manifest** | The Convox manifest file to use (defaults to `convox.yml`) |
| **Rack** | The Rack where review applications will be deployed |
| **Run tests** | Enable to run the test command specified in your `convox.yml` |
| **Deploy Demo** | Enable to create a temporary review application that will be deleted when the PR is merged |
| **Before Promote** | Specify a service and command to run before promotion (e.g., database migrations) |
| **After Promote** | Specify a service and command to run after promotion (e.g., notifications) |
| **Demo Environment** | Define environment variables for review applications |

### Review Workflow Behavior

When a review workflow is triggered:

1. Convox builds the application using the code from the pull request
2. If enabled, tests are run against the build
3. If enabled, a review application is created with a unique URL
4. The workflow status is reported back to GitHub/GitLab
5. When the PR is merged, the review application is automatically deleted

## Deployment Workflows

Deployment workflows automate the process of deploying your application to staging and production environments. They are triggered when code is merged into specified branches in your repository.

### Creating a Deployment Workflow

1. Navigate to the **Workflows** tab in the Convox Console
2. Click **Create Workflow**
3. Select **Deployment Workflow**
4. Configure the following settings:

| Setting | Description |
|---------|-------------|
| **Repository** | The connected GitHub or GitLab repository |
| **Workflow Name** | A descriptive name (e.g., "Staging Deploy") |
| **Branch** | The branch that triggers the workflow when code is merged |
| **Manifest** | The Convox manifest file to use (defaults to `convox.yml`) |
| **Applications** | One or more applications to deploy to, with promotion settings |
| **Run tests** | Enable to run the test command specified in your `convox.yml` |
| **Before Promote** | Specify a service and command to run before promotion |
| **After Promote** | Specify a service and command to run after promotion |
| **Environment** | Define environment variables for the deployment |

### Application Deployment Settings

For each application in a deployment workflow, you can specify:

- **Application**: Select from your existing Convox applications
- **Rack**: The Rack where the application will be deployed
- **Promotion**: Choose between:
  - **Automatic**: The new build is automatically promoted after creation
  - **Manual**: The build is created but must be manually promoted

### Manually Running a Workflow

You can manually trigger a deployment workflow at any time:

1. Navigate to the **Workflows** tab
2. Find the desired workflow
3. Click the **Play** button next to the workflow
4. Confirm to start the workflow

## Workflow Jobs

Each workflow run creates a job that can be monitored and managed through the Convox Console.

### Viewing Workflow Jobs

1. Navigate to the **Jobs** tab in the Convox Console
2. View all workflow jobs with their status and timestamps
3. Click on any job to see detailed logs and outcomes

### Job Details

The job details page provides:

- Step-by-step execution logs
- Build and release information
- Deployment URLs for review applications
- Error details if the workflow failed
- Options to re-run the job

## Command Line Interface

You can also manage workflows via the Convox CLI:

### List Workflows

```bash
$ convox workflows
ID                                    KIND        NAME
55dd9440-eb98-4d9b-816f-9923ee77feff  deployment  deployweb-app
c828b45a-070b-46ed-9c43-ddaa905ecd68  review      review-web-app
```

### Trigger a Workflow Run

```bash
$ convox workflows run 55dd9440-eb98-4d9b-816f-99230077feff --branch feat-branch --title "title"
Successfully trigger the workflow, job id: 65a4160a-27cd-47c6-ba74-aaaaaaaa
```

## Common Workflow Patterns

### Review Workflow for Pull Request Testing

This workflow automatically deploys a temporary version of your application for each pull request:

1. Create a review workflow for your repository
2. Assign it to your development or staging rack
3. Enable "Run tests" and "Deploy demo"
4. Configure any necessary environment variables

When a pull request is opened, a fully functional version of your application is deployed with the changes from the PR, allowing thorough testing before merging.

### GitFlow Deployment Strategy

For teams using GitFlow:

#### Staging Deployment
1. Create a deployment workflow for your `develop` branch
2. Configure it to deploy to your staging application with automatic promotion
3. Enable test running if desired

#### Production Deployment
1. Create a deployment workflow for your `master` branch
2. Configure it to deploy to your production application
3. Choose manual promotion for additional safety

### Build-Once Deploy-Many Strategy

This strategy builds your application once and deploys it to multiple environments:

1. Create a deployment workflow for your `master` branch
2. Add your staging application with automatic promotion
3. Add your production application with manual promotion

With this approach, code merged to master is automatically deployed to staging and simultaneously prepared for production. Once validated in staging, you can promote the identical build to production with one click.

## Best Practices

- **Use Clear Naming**: Name workflows and jobs descriptively to make them easy to identify
- **Leverage Environment Variables**: Use different environment configurations for review, staging, and production deployments
- **Implement Before/After Commands**: Use these hooks for database migrations and post-deployment tasks
- **Combine with Feature Branches**: Create a workflow that matches your team's branching strategy
- **Monitor Job Logs**: Regularly check job logs to identify and resolve issues quickly
- **Consider Resource Usage**: Review applications can consume significant resources; set appropriate scaling limits
- **Test Workflows**: Manually trigger workflows after creation to ensure they work as expected

By effectively using Convox workflows, you can automate your entire development pipeline from pull request to production, ensuring consistent testing and deployment processes across your applications.
