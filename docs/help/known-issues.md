---
title: "Known Issues"
slug: known-issues
url: /help/known-issues
---
# Known Issues

## Racks

### AWS

* EKS Node groups currently leak ENIs when they are being destroyed. This may cause failures when
  uninstalling AWS Racks. To work around this issue you must manually delete the ENIs in the VPC
  created for the Rack that are left behind in an "available" state.
  [aws/amazon-vpc-cni-k8s#608](https://github.com/aws/amazon-vpc-cni-k8s/issues/608)
  * Update: Convox has applied a fix that extends the delete operation timeout for public and private subnets (6-hour timeout plus a 5-minute delay). This works around the issue in most cases, but the underlying AWS ENI/subnet deletion race condition has not been fixed by AWS. Users may still encounter this in edge cases during rack uninstallation.

### Local

* The local development Rack uses self-signed TLS certificates. Your browser will show a certificate warning when accessing applications. This is expected behavior for local development.
* On macOS, you must keep `minikube tunnel` running in a separate terminal to reach the Rack. If the tunnel is stopped, CLI commands and browser access will fail until it is restarted.
