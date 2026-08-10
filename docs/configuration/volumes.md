---
title: "Volumes"
description: "Convox supports several volume types for persistent and temporary data, including Azure Files NFS shares with ReadWriteOnce, ReadOnlyMany, and ReadWriteMany."
slug: volumes
url: /configuration/volumes
---

# Volumes

Convox supports multiple types of volumes to manage both persistent and temporary data for your applications. These volumes provide flexibility for different use cases, from high-speed temporary data storage to persistent, scalable file storage across multiple services.

## Per-replica persistent volumes

Use a stateful service when each replica needs its own persistent disk. Convox creates a Kubernetes StatefulSet and one PersistentVolumeClaim for each replica. Kubernetes reattaches the same claim when it replaces or reschedules that replica.

```yaml
services:
  database:
    image: qdrant/qdrant:v1.18.2
    stateful: true
    podManagementPolicy: Parallel
    scale:
      count: 3
    volumeOptions:
      - persistentVolumeClaim:
          id: data
          mountPath: /qdrant/storage
          size: 200Gi
          storageClass: gp3
```

The rack must provide the named Container Storage Interface storage class. For example, use an Amazon EBS, Azure Disk or Google Persistent Disk storage class. The access mode defaults to `ReadWriteOnce`.

`podManagementPolicy` defaults to `OrderedReady`. Set it to `Parallel` when every replica must start independently, as Qdrant requires.

Stateful services must use a fixed replica count. They do not support agents, autoscaling, VPA, budget auto-shutdown or `convox run --use-service-volume`.

Changing `stateful` on an existing service does not provide an in-place migration. Create a new service and move the data. Kubernetes also treats `podManagementPolicy` and claim template fields such as `size` and `storageClass` as immutable, so changing one of those on a running stateful service is rejected. Use a new service name instead, which gives the replicas new claims.

### Addressing an individual replica

Each replica gets a stable name and its own DNS record through a headless Service that Convox creates alongside the StatefulSet:

```
<service>-<ordinal>.<service>-headless
```

The example above is reachable inside the app as `database-0.database-headless`, `database-1.database-headless` and `database-2.database-headless`. Use these names to point clustered services such as Qdrant at their peers. The regular `<service>` name still load balances across every ready replica.

### Volume ownership for non-root images

A newly provisioned block volume is owned by root, so a service whose image runs as a non-root user cannot write to its mount. Set `securityContext.fsGroup` to the group that owns the data directory and Kubernetes hands the volume to that group:

```yaml
services:
  search:
    image: elasticsearch:8.15.0
    stateful: true
    securityContext:
      fsGroup: 1000
    volumeOptions:
      - persistentVolumeClaim:
          id: data
          mountPath: /usr/share/elasticsearch/data
          size: 50Gi
          storageClass: gp3
```

An init container can also mount the same claim to prepare the directory before the service starts. Reference the claim by `id` and give it the path the init container should see:

```yaml
    initContainer:
      image: busybox
      command: chown -R 1000:1000 /data
      volumeOptions:
        - persistentVolumeClaim:
            id: data
            mountPath: /data
```

### Volume lifecycle

Claims outlive the replicas that use them. Scaling a stateful service down keeps the claims for the removed replicas, so scaling back up reattaches the same data. Removing the service from `convox.yml` also leaves its claims in place. In both cases the underlying cloud disks keep billing until you delete them, and `convox apps delete` removes the whole namespace along with every claim and disk it holds.

## Azure Files Volumes

> Azure only. Requires the [azure_files_enable](/configuration/rack-parameters/azure/azure_files_enable) rack parameter.

Azure Files provides a scalable, persistent NFS storage solution that allows multiple Convox services to access the same file system simultaneously. Azure Files volumes use a Premium FileStorage account with the NFS protocol for high-performance shared storage.

### Supported Access Modes

- **ReadWriteOnce (RWO)**: Single-service write operations. Each service has dedicated write access to its own files.
- **ReadOnlyMany (ROM)**: Multiple-service read operations. Suitable for distributing read-only content like model weights or configuration files across services.
- **ReadWriteMany (RWM)**: Multi-service read/write operations. Useful for shared file access among multiple services.

### Enabling Azure Files Volumes

To use Azure Files volumes, you must enable Azure Files on your rack. Run the following command to enable it:

```bash
convox rack params set azure_files_enable=true -r rackName
```

### Configuring Azure Files Volumes in convox.yml

After enabling the feature, define your Azure Files volumes in the `convox.yml` file:

```yaml
environment:
  - PORT=3000
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

- **azureFiles.id**: A unique identifier for the volume.
- **azureFiles.accessMode**: Specifies ReadWriteMany, ReadOnlyMany, or ReadWriteOnce.
- **azureFiles.mountPath**: Defines the mount point for the volume inside the service.
- **azureFiles.shareSize**: (Optional) The size of the NFS share. Defaults to 100Gi. Azure Premium Files has a minimum share size of 100GiB.

### Best Practices and Use Cases for Azure Files Volumes

Azure Files volumes are ideal for:

- **Shared Storage**: Ensures data is accessible to multiple service replicas.
- **Persistent Storage Across Restarts**: Maintains data persistence even after service restarts or scaling events.
- **ML Model Storage**: Store large model weights on a shared NFS volume accessible by all replicas without downloading on each startup.
- **Content Management Systems**: Allows multiple editors to access and modify shared content.

## AWS EFS Volumes

> AWS only. Requires the [efs_csi_driver_enable](/configuration/rack-parameters/aws/efs_csi_driver_enable) rack parameter.

AWS EFS (Elastic File System) provides a scalable, persistent storage solution that allows multiple Convox services to access the same file system simultaneously. EFS volumes are useful for applications that require shared access to files and need persistent data storage across services and restarts.

### Supported Access Modes

- **ReadWriteOnce (RWO)**: Single-service write operations. Each service has dedicated write access to its own files.
- **ReadOnlyMany (ROM)**: Multiple-service read operations. Suitable for distributing read-only content like configuration files across services.
- **ReadWriteMany (RWM)**: Multi-service read/write operations. Useful for shared file access among multiple services.

### Enabling AWS EFS Volumes

To use AWS EFS volumes, you must enable the EFS CSI driver on your rack. Run the following command to enable it:

```bash
convox rack params set efs_csi_driver_enable=true -r rackName
```

### Configuring AWS EFS Volumes in convox.yml

After enabling the driver, define your EFS volumes in the `convox.yml` file:

```yaml
environment:
  - PORT=3000
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

- **awsEfs.id**: The EFS volume ID.
- **awsEfs.accessMode**: Specifies ReadWriteMany, ReadOnlyMany, or ReadWriteOnce.
- **awsEfs.mountPath**: Defines the mount point for the volume inside the service.
- **awsEfs.volumeHandle**: (Optional) Specifies an existing EFS access point handle (format: `fs-id::fsap-id`). Use this to mount a pre-existing EFS access point instead of dynamically provisioning one.

### AWS EFS Storage Classes

You can specify a custom storage class for your EFS volumes. This provides greater flexibility for defining AWS storage behaviors and allows custom storage policies for specific workloads.

```yaml
environment:
  - PORT=3000
services:
  web:
    build: .
    port: 3000
    volumeOptions:
      - awsEfs:
          id: "data"
          accessMode: ReadWriteMany
          mountPath: "/opt/data/"
          storageClass: "efs-sc-33"
```

- **awsEfs.storageClass**: (Optional) Specifies the AWS EFS storage class to use for the volume. This allows you to apply custom storage policies and integrate with your organization's storage management requirements.

### Best Practices and Use Cases for AWS EFS Volumes

AWS EFS volumes are ideal for:

- **Shared Storage**: Ensures data is accessible to multiple services.
- **Persistent Storage Across Restarts**: Maintains data persistence even after service restarts.
- **Content Management Systems**: Allows multiple editors to access and modify shared content.
- **Data Processing**: Enables distributed data processing across multiple services.
- **Custom Storage Policies**: With storage class support, you can implement organization-specific storage policies.

## emptyDir Volumes

**emptyDir** volumes provide a temporary storage solution within your Convox services. These volumes are initially empty when a service starts and are removed when the service is terminated or rescheduled. **emptyDir** volumes are suited for storing non-persistent, ephemeral data.

### Configuring emptyDir Volumes in convox.yml

You can configure **emptyDir** volumes directly in the `convox.yml` file. Here's an example:

```yaml
environment:
  - PORT=3000
services:
  app:
    build: .
    port: 3000
    volumeOptions:
      - emptyDir:
          id: "my-vol1"
          mountPath: "/data"
      - emptyDir:
          id: "my-vol2"
          mountPath: "/data2"
          medium: Memory
```

In this configuration:

- **emptyDir.id**: The identifier for the volume.
- **mountPath**: Specifies where the volume is mounted in the service.
- **medium**: (Optional) Allows setting the volume medium to either the local disk (default) or `Memory` for RAM-based storage.

### Use Cases for emptyDir Volumes

**emptyDir** volumes are ideal for:

- **Temporary Data Storage**: Useful for non-persistent data that is required only for the lifespan of the service.
- **High-Speed Access**: When using `Memory` as the medium, it can be used for high-speed access to temporary data.

## See Also

- [convox.yml](/configuration/convox-yml) for the full configuration reference
- [Scaling](/configuration/scaling) for how volumes interact with scaling
