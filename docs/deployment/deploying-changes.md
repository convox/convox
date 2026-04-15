---
title: "Deploying Changes"
slug: deploying-changes
url: /deployment/deploying-changes
---
# Deploying Changes

To deploy changes to your code you will first create a [Release](/reference/primitives/app/release)
and then promote it.

Promoting a Release will begin a [rolling deployment](/deployment/rolling-updates) that will continue
until the new Release is active on all [Processes](/reference/primitives/app/process) or
has been completely rolled back.

## One Step

To create a [Release](/reference/primitives/app/release) and promote it in one step, use `convox deploy`:
```bash
    $ convox deploy -a myapp
    Packaging source... OK
    Uploading source... OK
    Starting build... OK
    ...
    Build: BABCDEFGHI
    Release: RBCDEFGHIJ
    Promoting RBCDEFGHIJ...
    2026-03-18T14:30:49Z system/k8s/atom/app Status: Running => Pending
    2026-03-18T14:30:51Z system/k8s/web Scaled up replica set web-745f845dc to 1
    2026-03-18T14:30:51Z system/k8s/web-745f845dc Created pod: web-745f845dc-rzl2q
    2026-03-18T14:30:51Z system/k8s/web-745f845dc-rzl2q Successfully assigned convox-myapp/web-745f845dc-rzl2q to instance-0a1b2c3d4e5f
    2026-03-18T14:30:51Z system/k8s/web-745f845dc-rzl2q Pulling image "registry.host/convox/myapp:web.BABCDEFGHI"
    2026-03-18T14:30:53Z system/k8s/atom/app Status: Pending => Updating
    2026-03-18T14:30:55Z system/k8s/web-745f845dc-rzl2q Successfully pulled image "registry.host/convox/myapp:web.BABCDEFGHI"
    2026-03-18T14:30:56Z system/k8s/web-745f845dc-rzl2q Created container main
    2026-03-18T14:30:56Z system/k8s/web-745f845dc-rzl2q Started container main
    OK
```
## Two Steps

You can also perform the steps of creating the [Release](/reference/primitives/app/release) and
promoting it with two different commands. This is useful if you would like to make changes to
[Environment Variables](/configuration/environment) or [run migrations](/management/run)
against the new [Release](/reference/primitives/app/release) before it is pushed live.
```bash
    $ convox build -a myapp
    Packaging source... OK
    Uploading source... OK
    Starting build... OK
    ...
    Build: BABCDEFGHI
    Release: RBCDEFGHIJ

    $ convox releases promote RCDEFGHIJK -a myapp
    Promoting RCDEFGHIJK...
    2026-03-18T14:30:49Z system/k8s/atom/app Status: Running => Pending
    2026-03-18T14:30:51Z system/k8s/web Scaled up replica set web-745f845dc to 1
    2026-03-18T14:30:51Z system/k8s/web-745f845dc Created pod: web-745f845dc-rzl2q
    2026-03-18T14:30:51Z system/k8s/web-745f845dc-rzl2q Successfully assigned convox-myapp/web-745f845dc-rzl2q to instance-0a1b2c3d4e5f
    2026-03-18T14:30:51Z system/k8s/web-745f845dc-rzl2q Pulling image "registry.host/convox/myapp:web.BABCDEFGHI"
    2026-03-18T14:30:53Z system/k8s/atom/app Status: Pending => Updating
    2026-03-18T14:30:55Z system/k8s/web-745f845dc-rzl2q Successfully pulled image "registry.host/convox/myapp:web.BABCDEFGHI"
    2026-03-18T14:30:56Z system/k8s/web-745f845dc-rzl2q Created container main
    2026-03-18T14:30:56Z system/k8s/web-745f845dc-rzl2q Started container main
    OK
```

## External Builds

By default, `convox build` and `convox deploy` package your entire source directory into a tarball and upload it to the rack for in-cluster building. For applications with large source directories (e.g., machine learning model weights, large binary assets), this upload can be slow or hit load balancer timeout limits.

The `--external` flag changes this flow by building the Docker image **locally** and pushing it directly to the rack's container registry:

```bash
    convox build --external -a myapp
    convox deploy --external -a myapp
```

With external builds:

1. The CLI creates a build record on the rack (a small API call)
2. The rack returns container registry credentials
3. Docker builds the image locally using your source directory
4. The CLI pushes the built image directly to the rack's container registry (ECR on AWS, ACR on Azure)
5. A release is created on the rack

The source tarball never passes through the rack's load balancer, eliminating upload size and timeout constraints. This approach also benefits from local Docker layer caching for faster rebuilds.

**Requirements:**

- Docker must be installed and running on the build machine
- The build machine must have network access to the rack's container registry

External builds work on all cloud providers (AWS, Azure, GCP) and are well-suited for CI/CD pipelines. See the [build](/reference/cli/build#external-builds) CLI reference for more details.

## Troubleshooting Failed Deployments

If a deployment fails or hangs, use `convox deploy-debug` to diagnose the issue:

```bash
    convox deploy-debug -a myapp
```

This command inspects your app's pods and provides actionable hints for common failure states like crash loops, image pull errors, OOM kills, and health check failures. See the [deploy-debug](/reference/cli/deploy-debug) reference for details.

## See Also

- [Rolling Updates](/deployment/rolling-updates) for how Convox handles zero-downtime deployments
- [Rollbacks](/deployment/rollbacks) for reverting to a previous release
- [CI/CD Workflows](/deployment/workflows) for automating deployments
