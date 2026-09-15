---
title: "Troubleshooting"
description: "Diagnose common Convox problems including install errors, failed deploys, SSL and health check failures, stuck uninstalls, failing Rack updates, and disappearing environment variables."
slug: troubleshooting
url: /help/troubleshooting
---
# Troubleshooting

## I got an error while installing Convox locally

Ensure you have followed the setup instructions for your local OS [here](/installation/development-rack). Memory requirements for running Docker/Kubernetes locally can often catch people out.

If you have an existing DNS service running on port 53 on your machine, that can conflict when trying to set up the local DNS resolution for your development Rack. Disabling the service during installation and forwarding traffic for `*.convox` domains should help.

If you have a process running on port 80 or 443 on your machine, that can conflict when trying to set up the Load Balancer for your development Rack. Disabling the service or using a different port will fix the problem.

## I got an error while installing Convox in my cloud provider

The error message is usually quite informative. If you have existing resources running in your cloud provider and you are near your account limits, a Rack install can sometimes breach those limits, requiring you to request an increase in the appropriate resource (IP addresses, CPU allowances etc). Once your limit has been increased, the Rack should install successfully.

Sometimes the Cloud providers will have internal issues which can prevent certain actions. It's always worth checking the status pages and reporting for your provider and retrying an install once the issue has been resolved.

- [AWS](https://status.aws.amazon.com/)
- [Azure](https://status.azure.com/en-us/status)
- [Digital Ocean](https://status.digitalocean.com/)
- [GCP](https://status.cloud.google.com/)

A failed Rack install should either be able to be continued with, or be uninstalled successfully as required. You can retry the installation by running `convox rack update -r <rackname>`.
If your Rack consistently fails to install into your Cloud provider and there are no relevant issues reported raise a [support ticket](/help/support) for us to investigate.

## I get an error when I deploy my app to Convox

Start with `convox deploy-debug -a myapp` to get pod-level diagnostics with actionable hints. This command inspects your app's Kubernetes pods and maps failure states to plain-language explanations, without requiring kubectl access. See the [deploy-debug](/reference/cli/deploy-debug) reference for full details.

You can also check your Application logs with the `convox logs` command. Use the `--filter` and `--since` flags to help narrow down your output if needed.

You can view the logs from your App build process with `convox builds logs <build>` if you are having issues at that stage.

Finally you can view the underlying Rack logs with `convox rack logs` to look for information around scaling or other external events.

When you know there is an issue and want to stop a deployment, you can run the `convox apps cancel` command. This will trigger an immediate rollback so you can fix the problem and try another deployment.

### SSL Certificate Issues

Convox uses LetsEncrypt to automatically provision SSL certificates for your App's domains if needed. In order for the provisioning to be successful, the DNS records for all the domains you list in your `convox.yml` for your App must resolve to the router address for your Rack. If you get a certificate warning and see the certificate is listed as a "Kubernetes Ingress Controller Fake Certificate", the certificate has not been issued, most likely because of DNS resolution issues. Correct the DNS records for those domains.

A certificate waiting on DNS has not failed, so no retry schedule is running. The Rack holds the challenge open and rechecks it, and the certificate is issued once the records resolve to the router address. Redeploying the App is not required.

When Let's Encrypt rejects a request, the issuance does fail, and the Rack retries it 5 minutes later, then 10, 20, and 40 minutes, doubling to a maximum of 32 hours. Rack version `3.25.7` and later use the 5 minute first retry; earlier Racks wait one hour.

### Health Checks

Any Services within your App that expose a port will require a passing [health check](/configuration/health-checks) before receiving traffic. Deploying a Release of your App that does not pass the health checks will result in a rollback to the previous release. If this is your first release of a new App, a failing health check will result in a failed deployment.
Failing health checks will be reported when promoting your Release:
```text
Promoting RABCDEFGHIJ...
2026-03-18T14:16:50Z system/k8s/atom/app Status: Running => Pending
2026-03-18T14:16:53Z system/k8s/atom/app Status: Pending => Updating
2026-03-18T14:16:53Z system/k8s/web-56f5d77d7 Created pod: web-56f5d77d7-6gx8z
2026-03-18T14:16:53Z system/k8s/web Scaled up replica set web-56f5d77d7 to 1
2026-03-18T14:16:53Z system/k8s/web-56f5d77d7-6gx8z Successfully assigned abcde-myapp/web-56f5d77d7-6gx8z to gke-abcde-abcde-nodes-n1-highcpu-8-90530fd3-p77q
2026-03-18T14:16:54Z system/k8s/web-56f5d77d7-6gx8z Pulling image "gcr.io/abcde-123456/myapp:web.BABCDEFGHIJ"
2026-03-18T14:17:06Z system/k8s/web-56f5d77d7-6gx8z Successfully pulled image "gcr.io/abcde-123456/myapp:web.BABCDEFGHIJ"
2026-03-18T14:17:09Z system/k8s/web-56f5d77d7-6gx8z Started container main
2026-03-18T14:17:09Z system/k8s/web-56f5d77d7-6gx8z Created container main
2026-03-18T14:17:17Z system/k8s/web-56f5d77d7-6gx8z Readiness probe failed: HTTP probe failed with statuscode: 404
```
You should ensure that your App is able to respond to the health check probes to facilitate a successful deployment.

## My app deployed but I cannot access it

Run `convox services` to find the load balancer endpoints for your application.

Run `convox ps` to determine if your application is booting successfully.

Run `convox logs` to inspect your application logs and cluster events for problems placing your container, starting your app, or registering with the load balancer.

## My app stopped working and I want to restart it

You can perform a remote restart of an entire App (all running processes) from the CLI with:

```sh
$ convox restart -a app1
```

Or alternatively to restart the `web` service processes, you can perform:

```sh
$ convox services restart web -a app1
```

## My CLI commands take a long time to return

If your local Kubernetes setup does not point to a valid cluster, that can slow down your Convox CLI operations as it tries to interrogate the invalid endpoint. In this case, you can set a local env var `$ export CONVOX_LOCAL=disable` to stop the CLI from doing this and speed up your commands.

## A rack uninstall is stuck or failed

When a rack uninstall fails, it is typically because a cloud resource cannot be deleted due to dependencies.

1. **Attempt a forced uninstall from the CLI.**
    ```bash
$ convox rack uninstall <rack_name>
    ```

2. **Identify the blocking resource in CloudFormation (AWS).**
    If the CLI command fails, go to the AWS CloudFormation console. Find the stack for your rack (e.g., `convox-my-rack`). Its status will be `DELETE_FAILED`. Check the "Events" or "Resources" tab to find the specific resource that failed to delete.

3. **Manually delete the blocking resource.**
    Navigate to the appropriate service console (e.g., EC2 for Network Interfaces, VPC for Security Groups) and delete the resource identified in the previous step. You may need to detach or delete other dependent resources first.

4. **Retry the CloudFormation stack deletion.**
    Return to the CloudFormation console, select the failed stack, and click "Delete". In the deletion dialog, check the box next to the resource you manually deleted and proceed.

Once the CloudFormation stack is gone, the rack will be removed from your Convox console.

## A Rack update keeps failing with a Helm "another operation in progress" error

Every attempt at the same `convox rack update` or `convox rack params set` fails on the same resource, naming a `helm_release` and the module file it is declared in:

```text
Error: another operation (install/upgrade/rollback) is in progress
```

This happens when an earlier apply was killed while Helm was in the middle of an operation, for example an update that was cancelled or interrupted while it was running. The release is left in a pending state (`pending-install`, `pending-upgrade`, or `pending-rollback`) and Helm refuses to operate on it again, so retrying the identical command produces the identical error indefinitely.

**Resolution:** re-run the same command with a `convox` binary that carries the check:

| Rack | Minimum `convox` version |
|------|--------------------------|
| Public Kubernetes API endpoint | `3.25.3` |
| Private endpoint host | `3.25.5` |

Convox clears the stranded revision first and prints a `NOTICE` naming the release, its status, and its revision:

```text
NOTICE: cleared stuck Helm release karpenter (pending-upgrade, revision 4) before apply
```

The `NOTICE` prints after the revision is gone, so it reports a clear that completed rather than one that was attempted.

Only the pending revision is removed. The revision Helm still considers deployed stays current, and the apply that follows retries the operation.

The check is best effort and never fails the apply. On a Rack reached through a private endpoint host, two more messages report what it could not do:

```text
NOTICE: skipping stuck Helm release check, could not reach the cluster
```

```text
NOTICE: could not confirm clearing of stuck Helm release karpenter (pending-upgrade, revision 4)
```

The first prints when Convox cannot list the cluster's Helm release secrets. The second prints when the delete itself fails. Neither appears on a Rack with a public endpoint, where a failed clear prints nothing.

The scope of this recovery is deliberately narrow:

- AWS Racks only. It does not run on GCP, Azure, Digital Ocean, Equinix Metal, or Local Racks.
- Racks whose Kubernetes API is reached through a private endpoint host are covered from `3.25.5`. Earlier versions skip them.
- Convox-owned releases only, matched on both release name and namespace: `aws-lbc`, `karpenter`, `karpenter-crd`, `keda`, `vpa`, `dcgm-exporter`, `nvidia-device-plugin`, `contour`, and `contour-internal`. Helm releases you installed yourself are never touched, and neither is a release that shares one of those names in a namespace Convox does not own.
- Only releases that have been stranded for more than fifteen minutes, so an apply that is still working is never interrupted.
- Only when the `convox` binary running the apply meets the version in the table above. For a self-managed Rack that is the CLI on your machine. For a Console-managed Rack it is the CLI bundled in the Console deploy, which means a Rack version update on its own does not deliver it. See [CLI Rack Management](/management/cli-rack-management) for the full version rule.

If a re-run fails the same way, open a [support ticket](/help/support) with the Rack logs.

## Raising a node group minimum fails EKS validation

Increasing `min_size` on an entry in [additional_node_groups_config](/configuration/rack-parameters/aws/additional_node_groups_config) fails the apply with an EKS validation error:

```text
Error: updating EKS Node Group (my-rack:my-rack-additional-0-a1b2c3d4e5f60718) config: operation error EKS: UpdateNodegroupConfig, https response error StatusCode: 400, InvalidParameterException: Minimum capacity 3 can't be greater than desired size 1
```

Convox does not manage the running size of these node groups, so the autoscaler is free to move them. When a group has already scaled below the new floor, the update carries the new minimum on its own, and EKS rejects a minimum that is above the group's current desired size.

The same error appears when raising [build_node_min_count](/configuration/rack-parameters/aws/build_node_min_count), [min_on_demand_count](/configuration/rack-parameters/aws/min_on_demand_count) on a Rack with `node_capacity_type=mixed`, or [karpenter_system_node_min_count_per_az](/configuration/rack-parameters/aws/karpenter_system_node_min_count_per_az) on a Rack with Karpenter enabled, above the number of nodes that group is currently running.

**Resolution:** re-run the same command with a `convox` binary that carries the handling for the group you raised:

| Node group | Minimum `convox` version |
|------------|--------------------------|
| Additional node groups (`additional_node_groups_config`) | `3.25.3` |
| Build node group (`build_node_min_count`) | `3.25.6` |
| On-demand system node group (`min_on_demand_count`, `node_capacity_type=mixed`) | `3.25.6` |
| System node groups (`karpenter_system_node_min_count_per_az`, `karpenter_enabled=true`) | `3.25.7` |

Convox raises the group's desired size to the new minimum first, waits for that scale-up to finish, and then applies, so the command succeeds. It prints a `NOTICE` for each group it raises:

```text
NOTICE: raising node group my-rack-additional-0-a1b2c3d4e5f60718 desired size to 3 before apply
```

Two things follow from this:

- The update takes longer than usual when the minimum jumps by a large amount, because the nodes are added before Terraform runs.
- The same version condition applies as above. The behavior lives in the `convox` binary performing the apply, which for a Console-managed Rack means a Console deploy carrying that CLI version, not a Rack version update.

This handling applies to AWS Racks. With [`karpenter_enabled`](/configuration/rack-parameters/aws/karpenter_enabled) set to `false` it covers `additional_node_groups_config`, the build node group and the on-demand system node group. With it set to `true` it covers `additional_node_groups_config` and, from `3.25.7`, the per-zone system node groups; the build node group and the on-demand system node group are not raised.

## Downgrading a Karpenter Rack fails on a node group maximum

A [Karpenter](/configuration/scaling/karpenter) Rack whose system node groups run more than ten nodes, or whose build node group runs more than one, fails to apply a Rack version before `3.25.7`:

```text
Error: updating EKS Node Group (my-rack:my-rack-us-east-2a-0a1b2c3d4e5f60718) config: operation error EKS: UpdateNodegroupConfig, https response error StatusCode: 400, InvalidParameterException: desired capacity 11 can't be greater than max size 10
```

Versions before `3.25.7` cap each system node group at 10 nodes and the build node group at 1 while Karpenter is enabled, and EKS rejects a maximum below a group's recorded desired size.

The apply can run for half an hour before it fails, and the Rack is left reporting the version it was moving to, with the node group un-updated. `convox rack` showing the older version after this error is not evidence that the downgrade finished. Apply the resolution below and run the downgrade again.

**Resolution:** bring the running counts down first, then downgrade. Lowering [karpenter_system_node_min_count_per_az](/configuration/rack-parameters/aws/karpenter_system_node_min_count_per_az) is not enough on its own, because Convox does not lower a node group's running size. Set it to `10` or less, then change [node_disk](/configuration/rack-parameters/aws/node_disk) or [node_type](/configuration/rack-parameters/aws/node_type) to a different value:

```bash
$ convox rack params set karpenter_system_node_min_count_per_az=1 -r rackName
Updating parameters... OK
```

Wait for that update to finish, then:

```bash
$ convox rack params set node_disk=30 -r rackName
Updating parameters... OK
```

A `node_disk` change replaces the system node groups and the build node group together. A `node_type` change replaces the system node groups, and the build node group as well when [build_node_type](/configuration/rack-parameters/aws/build_node_type) is unset, because the build node group then takes its instance type from `node_type`. Where `build_node_type` is set, change it to a different instance type to replace the build node group; setting it to the type the group already runs leaves it in place.

Convox creates the new groups at the configured count, then drains and deletes every node in the old ones in the same apply, so every Rack component running on a system node is rescheduled. Terraform allows one hour to create the new groups and one hour to delete the old ones. The downgrade then applies. Changing `node_capacity_type` does not clear it, because Karpenter already forces that value to `ON_DEMAND`.

## A Rack update fails with a Terraform error after setting a parameter

`convox rack params set` returns `Updating parameters... OK` and the Rack update then fails before Terraform reaches a plan:

```text
Error: Missing newline after argument

  on main.tf line 89, in module "system":
  89:     user_data = "echo "hello"
```

`Error: Invalid multi-line string` and `Error: Invalid escape sequence` are the same problem. A `convox` binary older than `3.25.7` cannot carry a parameter value containing a double quote, a line break, a backslash or a `${` sequence into the Rack's configuration. [user_data](/configuration/rack-parameters/aws/user_data) is where this comes up most often. It is not specific to AWS: the same limit applies on GCP, Azure and Digital Ocean Racks.

**Resolution:** run [`sudo convox update`](/reference/cli/update) and re-run the command. The version condition is on the `convox` binary performing the apply, not on the Rack. For a Console-managed Rack that binary is the CLI bundled in the Console deploy, so a Rack version update on its own does not deliver it. See [CLI Rack Management](/management/cli-rack-management).

On a self-managed Rack the value is stored before the Rack's configuration is built, so `convox rack params` lists a value that never applied. Once the CLI is updated, the next Rack update applies that value along with whatever you changed in that command.

## A Build stays running and never finishes

`convox builds` shows the Build as `running` long past the time the App normally takes to build, and `convox builds logs` returns `unable to read logs for build: <build>`.

A build pod that cannot be scheduled leaves the Build at `running` indefinitely. Nothing on the Rack times it out. The usual cause is a placement constraint no node satisfies: [BuildLabels](/configuration/app-parameters/aws/BuildLabels) naming labels the cluster does not carry, or [BuildArch](/configuration/app-parameters/aws/BuildArch) selecting an architecture with no build node of that architecture on a Rack with [build_node_enabled](/configuration/rack-parameters/aws/build_node_enabled) set to `true`.

**Resolution:** clear the Build, then fix the constraint.

```bash
$ convox builds cancel BABCDEFGHIJ -a myapp
Cancelling build BABCDEFGHIJ... OK
```

Cancelling deletes the build pod and sets the Build to `failed`. It does not change the configuration that left the pod unschedulable, so correct `BuildLabels` or `BuildArch` before the next Build. See [builds](/reference/cli/builds#builds-cancel).

`convox builds cancel` requires Rack version `3.25.7` or later. The subcommand ships in the `convox` CLI at `3.25.7`, so run [`sudo convox update`](/reference/cli/update) if your CLI does not recognize it. This is a different command from [apps cancel](/reference/cli/apps#apps-cancel), which cancels a deploy in progress and does not affect Builds.

## My environment variables disappear after deploying

This happens because of how Convox manages state through [Releases](/reference/primitives/app/release). Running `convox env set` creates a new, unpromoted release. Running `convox deploy` creates a release based on the **currently active** release's environment, not the latest unpromoted one.

If you set an environment variable but deploy before promoting it, the deployment overwrites your change:

1. `R1` is active.
2. `convox env set FOO=bar` creates `R2` (not promoted). `R1` is still active.
3. `convox deploy` creates `R3` based on `R1`'s environment. `FOO=bar` is lost.

**To avoid this**, promote your environment changes before deploying:

```bash
$ convox env set FOO=bar -a myapp
# Creates release R2
$ convox releases promote R2 -a myapp
# Now R2 is active
$ convox deploy -a myapp
# New release inherits FOO=bar from R2
```

Or use the `--promote` flag to set and promote in one step:

```bash
$ convox env set FOO=bar --promote -a myapp
```

See [Environment Variables](/configuration/environment) for more details.

## Still Having Trouble?

- Search this site via the search box in the sidebar.
- Community support is available on [Stack Overflow](https://stackoverflow.com/questions/tagged/convox) using the `convox` tag.

## See Also

- [Known Issues](/help/known-issues) for a list of current known issues and workarounds
- [Direct Kubernetes Access](/management/direct-k8s-access) for advanced debugging via kubectl
- Open a ticket via the Support section [in the Convox web console](https://console.convox.com/)
