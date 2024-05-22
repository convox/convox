---
title: "efs_csi_driver_enable"
draft: false
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

## Managing Parameters

### Viewing Current Parameters
```html
$ convox rack params -r rackName
efs_csi_driver_enable  false
node_disk  20
node_type  t3.small
```

### Setting Parameters
To enable the EFS CSI driver, use the following command:
```html
$ convox rack params set efs_csi_driver_enable=true -r rackName
Setting parameters... OK
```
This command enables the AWS EFS volume feature for your rack.

## Additional Information
AWS EFS provides a scalable file storage solution that supports multiple instances accessing the same file system, making it ideal for shared data storage across service replicas. 

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
