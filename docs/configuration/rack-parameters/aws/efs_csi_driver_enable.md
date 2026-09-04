---
title: "efs_csi_driver_enable"
description: "The efs_csi_driver_enable AWS rack parameter enables the EFS CSI driver so services can use AWS EFS for scalable, shared file storage, defaulting to false."
slug: efs_csi_driver_enable
url: /configuration/rack-parameters/aws/efs_csi_driver_enable
---

# efs_csi_driver_enable

## Description
The `efs_csi_driver_enable` parameter enables the EFS CSI driver to use the AWS EFS volume feature. This allows your services to utilize AWS Elastic File System (EFS) for scalable, shared file storage.

## Default Value
The default value for `efs_csi_driver_enable` is `false`.

## Use Cases
- **Shared Service Volumes**: Enable multiple EC2 instances to access the same file system simultaneously, supporting access modes like ReadWriteOnce (RWO), ReadOnlyMany (ROM), and ReadWriteMany (RWM).
- **Enhanced Data Storage**: Use AWS EFS for applications requiring shared access to files across distributed instances.

## Setting Parameters
To enable the EFS CSI driver, use the following command:
```bash
$ convox rack params set efs_csi_driver_enable=true -r rackName
Updating parameters... OK
```
This command enables the AWS EFS volume feature for your rack.

### Disabling the Driver
Setting `efs_csi_driver_enable=false` destroys the EFS filesystem and everything stored on it. The data is not recoverable. Back up anything you need before disabling the parameter.

## Additional Information
This parameter is available on AWS Racks only. On a Rack installed with [private](/configuration/rack-parameters/aws/private) set to `false`, it requires Rack version `3.25.6` or later. Before `3.25.6` the Rack placed EFS mount targets in its private subnets, so a Rack that has none could not use EFS. `private` is fixed at install, so updating the Rack is the only route on an affected Rack.

AWS EFS provides a scalable file storage solution that supports multiple instances accessing the same file system, making it ideal for shared data storage across service replicas.

### Mount Targets
AWS allows one mount target per availability zone. A Rack whose subnets Convox created gets three, or two when [high_availability](/configuration/rack-parameters/aws/high_availability) is `false`. A Rack that supplied its own subnet ids gets one mount target per subnet in a single list, its private subnets when the Rack is `private` and its public subnets otherwise, and a list whose entries share an availability zone fails the apply. A Rack that supplied its own private subnet ids keeps its mount targets in those subnets whichever way `private` was set.

Each mount target's network interface takes a private VPC address in either subnet type, and NFS on TCP 2049 is reachable only from the cluster security group. A public subnet differs only in carrying an internet gateway route, which does not apply to an EFS network interface.

Downgrading a Rack that has no private subnets below `3.25.6` removes its mount targets. The filesystem, its contents and the storage classes survive, so no data is lost, but nothing can mount an EFS volume until the Rack is updated back to `3.25.6` or later. Disabling `efs_csi_driver_enable` does not recover from that state, and it deletes the filesystem and everything in it.

### Example Configuration
To configure your services to use AWS EFS for persistent storage, you can set up your `convox.yml` as follows:
```yaml
services:
  web:
    build: .
    port: 3000
    volumeOptions:
      - awsEfs:
          id: "efs-1"
          accessMode: ReadWriteMany
          mountPath: "/my/data/"
      - awsEfs:
          id: "efs-2"
          accessMode: ReadOnlyMany
          mountPath: "/my/read-only/data/"
```
Enabling the EFS CSI driver provides enhanced flexibility and scalability for your data storage needs, leveraging AWS EFS's capabilities for your applications.
