---
title: "azure_files_enable"
slug: azure_files_enable
url: /configuration/rack-parameters/azure/azure_files_enable
---

# azure_files_enable

## Description
The `azure_files_enable` parameter enables Azure Files NFS volumes for shared persistent storage across service replicas. This creates a Premium FileStorage account and an NFS StorageClass in your AKS cluster.

## Default Value
The default value for `azure_files_enable` is `false`.

## Use Cases
- **Shared Service Volumes**: Enable multiple service replicas to access the same file system simultaneously, supporting access modes like ReadWriteOnce (RWO), ReadOnlyMany (ROM), and ReadWriteMany (RWM).
- **Persistent Storage**: Use Azure Files for applications requiring shared access to files across distributed instances and restarts.
- **ML Model Storage**: Store large model weights on a shared NFS volume accessible by all replicas without downloading on each startup.

## Setting Parameters
To enable Azure Files volumes, use the following command:
```bash
$ convox rack params set azure_files_enable=true -r rackName
Setting parameters... OK
```
This command enables the Azure Files NFS volume feature for your rack.

## Additional Information
Azure Files provides a scalable file storage solution using the NFS protocol. It uses a Premium FileStorage account for the performance required by NFS shares, with a minimum share size of 100GiB.

AKS includes the Azure Files CSI driver (`file.csi.azure.com`) by default — no additional driver installation is needed.

### Example Configuration
To configure your services to use Azure Files for persistent storage, set up your `convox.yml` as follows:
```yaml
services:
  web:
    build: .
    port: 3000
    volumeOptions:
      - azureFiles:
          id: "shared-data"
          accessMode: ReadWriteMany
          mountPath: "/mnt/data/"
      - azureFiles:
          id: "models"
          accessMode: ReadOnlyMany
          mountPath: "/mnt/models/"
          shareSize: "200Gi"
```
Enabling Azure Files provides shared, persistent NFS storage for your applications running on Azure AKS.
