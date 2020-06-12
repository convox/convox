# Troubleshooting

## I got an error while installing Convox locally

Ensure you have followed the setup instructions for your local OS [here](../installation/development-rack).  Memory requirements for running Docker/Kubernetes locally can often catch people out.

If you have an existing DNS service running on port 53 on your machine, that can conflict when trying to set up the local DNS resolution for your development Rack. Disabling the service during installation and forwarding traffic for `*.convox` domains should help.

## I got an error while installing Convox in my cloud provider

The error message is usually quite informative.  If you have existing resources running in your cloud provider and you are near your account limits, a Rack install can sometimes breach those limits, requiring you to request an increase in the appropriate resource (IP addresses, CPU allowances etc). Once your limit has been increased, the Rack should install successfully.

Sometimes the Cloud providers will have internal issues which can prevent certain actions.  It's always worth checking the status pages and reporting for your provider and retrying an install once the issue has been resolved.

- [AWS](https://status.aws.amazon.com/)
- [Azure](https://status.azure.com/en-us/status)
- [Digital Ocean](https://status.digitalocean.com/)
- [GCP](https://status.cloud.google.com/)

A failed Rack install should either be able to continued with, or be uninstalled successfully as required.  You can retry the installation by simply running `convox rack update -r <rackname>`.
If your Rack consistently fails to install into your Cloud provider and there are no relevant issues reported then please raise a [support ticket](/help/support) for us to investigate.

Sometimes, a failed Rack install cannot be uninstalled due to cloud provider issues.  In this instance, it is best to remove any remaining resources yourself before re-attempting the Rack install.

- AWS: Delete the EKS cluster for your Rack (if it exists). You may have to delete any node groups attached to it first.  Delete the VPC created for your Rack. AWS will inform you if there are any child resources that need to be deleted first.
- Azure:
- Digital Ocean:
- Google Cloud:

## I get an error when I deploy my app to Convox

Sometimes the errors that come back from Kubernetes and/or the cloud providers are useful, sometimes they're more obtuse.
You can check your Application logs with the `convox logs` command.  Use the `--filter` and `--since` flags to help narrow down your output if needed!

You can view the logs from your App build process with `convox builds logs <build>` if you are having issues at that stage.

Finally you can view the underlying Rack logs with `convox rack logs` to look for information around scaling or other external events.

When you know there is an issue and want to stop a deployment, you can run the `convox apps cancel` command. This will trigger an immediate rollback so you can fix the problem and try another deployment.

### SSL Certificate Issues

Convox uses LetsEncrypt to automatically and seamlessly provision SSL certificates for your App's domains if needed. In order for the provisioning to be successful, the DNS records for all the domains you list in your `convox.yml` for your App must resolve to the router address for your Rack.  If you get a certificate warning and see the certificate is listed as a "Kubernetes Ingress Controller Fake Certificate", this means the provisioning has failed, most likely because of DNS resolution issues.  Check these and try again!

### Health Checks

Any Services within your App that expose a port will require a passing [health check](../configuration/health-checks) before receiving traffic.  Deploying a Release of your App that does not pass the health checks will result in a rollback to the previous release.  If this is your first release of a new App, a failing health check will result in a failed deployment.
Failing health checks will be reported when promoting your Release:

    Promoting RABCDEFGHIJ...
    2020-02-15T21:16:50Z system/k8s/atom/app Status: Running => Pending
    2020-02-15T21:16:53Z system/k8s/atom/app Status: Pending => Updating
    2020-02-15T21:16:53Z system/k8s/web-56f5d77d7 Created pod: web-56f5d77d7-6gx8z
    2020-02-15T21:16:53Z system/k8s/web Scaled up replica set web-56f5d77d7 to 1
    2020-02-15T21:16:53Z system/k8s/web-56f5d77d7-6gx8z Successfully assigned abcde-myapp/web-56f5d77d7-6gx8z to gke-abcde-abcde-nodes-n1-highcpu-8-90530fd3-p77q
    2020-02-15T21:16:54Z system/k8s/web-56f5d77d7-6gx8z Pulling image "gcr.io/abcde-123456/myapp:web.BABCDEFGHIJ"
    2020-02-15T21:17:06Z system/k8s/web-56f5d77d7-6gx8z Successfully pulled image "gcr.io/abcde-123456/myapp:web.BABCDEFGHIJ"
    2020-02-15T21:17:09Z system/k8s/web-56f5d77d7-6gx8z Started container main
    2020-02-15T21:17:09Z system/k8s/web-56f5d77d7-6gx8z Created container main
    2020-02-15T21:17:17Z system/k8s/web-56f5d77d7-6gx8z Readiness probe failed: HTTP probe failed with statuscode: 404

You should ensure that your App is able to respond to the health check probes to faciliate a successful deployment.

## My app deployed but I cannot access it

Run `convox services` to find the load balancer endpoints for your application.

Run `convox ps` to determine if your application is booting successfully.

Run `convox logs` to inspect your application logs and cluster events for problems placing your container, starting your app, or registering with the load balancer.

## My app stopped working and I want to restart it

You can perform a remote restart of an entire App (all running processes) from the CLI with:

```sh
$ convox restart -a app1
```

Or alternatively to just restart the `web` service processes, you can perform:

```sh
$ convox services restart web -a app1
```

## My CLI commands take a long time to return

If your local Kubernetes setup does not point to a valid cluster, that can slow down your Convox CLI operations as it tries to interrogate the invalid endpoint.  In this case, you can set a local env var `$ export CONVOX_LOCAL=disable` to stop the CLI from doing this and speed up your commands.

## Still having trouble?

Some good places to search are:

- this site, via the search box on in the sidebar
- Community support is available on [Stack Overflow](https://stackoverflow.com/questions/tagged/convox).

If you still need help, feel free to:

- post a question on the [Stack Overflow](https://stackoverflow.com/questions/tagged/convox) using the `convox` tag.
- open a ticket via the Support section [in the Convox web console](https://console.convox.com/)
