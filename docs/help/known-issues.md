# Known Issues

## Racks

### AWS

* EKS Node groups currently leak ENIs when they are being destroyed. This may cause failures when
  uninstalling AWS Racks. To work around this issue you must manually delete the ENIs in the VPC
  created for the Rack that are left behind in an "available" state.
  [aws/amazon-vpc-cni-k8s#608](https://github.com/aws/amazon-vpc-cni-k8s/issues/608) 