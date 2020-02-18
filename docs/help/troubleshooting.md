# Troubleshooting

## Local Rack

### Installation

Ensure you have followed the setup instructions for your local OS [here](../installation/development-rack).  Memory requirements for running Docker/Kubernetes locally can often catch people out.

## Cloud Rack

### Installation

One of the majoy advantages of Convox is that it abstracts you from the complexities involved in installing Kubernetes clusters on the cloud provider of your choice.  You are still subject, unfortunately, to any errors or issues that your provider is experiencing.  If encountering an error during installation, it's worth checking the Status page for your provider:

- [AWS](https://status.aws.amazon.com/)
- [GCP](https://status.cloud.google.com/)
- [Azure](https://status.azure.com/en-us/status)
- [Digital Ocean](https://status.digitalocean.com/)

And to retry your installation after a few minutes.  If your Rack consistently fails to install into your Cloud provider and there are no relevant issues reported then please raise a [support ticket](/help/support) for us to investigate.

### Scaling

Scaling issues can often arise when you run into quota limits set by your cloud provider.  Ensure you have adequate limits in place for your scaling needs by contacting your cloud provider's support to get them set appropriately.

## Running your Apps

### Health Checks

Any services within your app that expose a port will require a passing health check before receiving traffic.  Deploying a Release of your App that does not pass the health checks will result in a rollback to the previous release.  If this is your first release of a new app, a failing health check will result in a failed deployment.
Failing health checks will be reported when promoting your release:

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

You should ensure that your app is able to responsd to the health check probes to faciliate a successful deployment.
