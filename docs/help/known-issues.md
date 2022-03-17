---
title: "Known Issues"
draft: false
slug: Known Issues
url: /help/known-issues
---
# Known Issues

## Racks

### AWS

* EKS Node groups currently leak ENIs when they are being destroyed. This may cause failures when
  uninstalling AWS Racks. To work around this issue you must manually delete the ENIs in the VPC
  created for the Rack that are left behind in an "available" state.
  [aws/amazon-vpc-cni-k8s#608](https://github.com/aws/amazon-vpc-cni-k8s/issues/608)
  * Update:  We have provided a fix for this issue that extends the delete operation timeout 
    for public and private subnets.
    
### Local

 * Kubernetes >= 1.22 is not supported as it deprecated an Ingress resource name which Convox uses. We are going to deploy a fix, but meanwhile the workaround is to run version <= 1.21. Convox automatically uses a compatible version in remote racks, but you must ensure that you're running <= 1.21 locally.
   * macOS: Docker Desktop does not allow you to specify the Kubernetes version. The latest Docker Desktop version with Kubernetes <= 1.21 is [Docker Desktop 4.2.0](https://docs.docker.com/desktop/mac/release-notes/#docker-desktop-420) and there are alternatives such as [minikube](https://minikube.sigs.k8s.io/docs/).
