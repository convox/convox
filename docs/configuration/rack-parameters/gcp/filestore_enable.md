---
title: "filestore_enable"
slug: filestore_enable
url: /configuration/rack-parameters/gcp/filestore_enable
---

# filestore_enable

## Description
The `filestore_enable` parameter enables Google Cloud Filestore NFS volumes for shared persistent storage across service replicas. It enables the Filestore API and the GKE Filestore CSI driver, and creates a `filestore-rwx` StorageClass connected to the rack's VPC.

## Default Value
The default value for `filestore_enable` is `false`.

## Use Cases
- **Shared Service Volumes**: Let multiple service replicas read and write the same file system.
- **Persistent Storage**: Keep files across restarts, deploys and scaling events.
- **ML Model Storage**: Store large model weights once and mount them read-only in every replica.

## Setting Parameters
```bash
$ convox rack params set filestore_enable=true -r rackName
Setting parameters... OK
```

## Additional Information
Each `gcpFilestore` volume provisions its own basic HDD Filestore instance with a minimum capacity of 1TiB, billed separately.

Remove all `gcpFilestore` volumes before setting `filestore_enable=false`. Disabling the parameter removes the CSI driver and StorageClass, so existing volumes can no longer be mounted.

### Example Configuration
```yaml
services:
  web:
    build: .
    port: 3000
    volumeOptions:
      - gcpFilestore:
          id: "shared-data"
          accessMode: ReadWriteMany
          mountPath: "/mnt/data/"
```

See [Volumes](/configuration/volumes) for all options.
