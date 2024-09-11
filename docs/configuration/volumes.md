---
title: "Volumes"
draft: false
slug: volumes
url: /configuration/volumes
---

# Volumes

Convox supports multiple types of volumes to manage both persistent and temporary data for your applications. These volumes provide flexibility for different use cases, from high-speed temporary data storage to persistent, scalable file storage across multiple services.

## AWS EFS Volumes

AWS EFS (Elastic File System) provides a scalable, persistent storage solution that allows multiple Convox services to access the same file system simultaneously. EFS volumes are useful for applications that require shared access to files and need persistent data storage across services and restarts.

### Supported Access Modes

- **ReadWriteOnce (RWO)**: Single-service write operations. Each service has dedicated write access to its own files.
- **ReadOnlyMany (ROM)**: Multiple-service read operations. Suitable for distributing read-only content like configuration files across services.
- **ReadWriteMany (RWM)**: Multi-service read/write operations. Useful for shared file access among multiple services.

### Enabling AWS EFS Volumes

To use AWS EFS volumes, you must enable the EFS CSI driver on your rack. Run the following command to enable it:

```html
convox rack params set efs_csi_driver_enable=true -r rackName
```

### Configuring AWS EFS Volumes in convox.yml

After enabling the driver, define your EFS volumes in the `convox.yml` file:

```html
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
- **mountPath**: Defines the mount point for the volume inside the service.

### Best Practices and Use Cases for AWS EFS Volumes

AWS EFS volumes are ideal for:

- **Shared Storage**: Ensures data is accessible to multiple services.
- **Persistent Storage Across Restarts**: Maintains data persistence even after service restarts.
- **Content Management Systems**: Allows multiple editors to access and modify shared content.
- **Data Processing**: Enables distributed data processing across multiple services.

### Version Requirements for AWS EFS Volumes

You must be on at least rack version `3.18.2` to use AWS EFS volumes. If you are on an earlier version, update your rack using the following command:

For more details, refer to the [Updating a Rack](https://docs.convox.com/management/cli-rack-management/) documentation.

## emptyDir Volumes

**emptyDir** volumes provide a temporary storage solution within your Convox services. These volumes are initially empty when a service starts and are removed when the service is terminated or rescheduled. **emptyDir** volumes are suited for storing non-persistent, ephemeral data.

### Configuring emptyDir Volumes in convox.yml

You can configure **emptyDir** volumes directly in the `convox.yml` file. Here’s an example:

```html
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

### Version Requirements for emptyDir Volumes

You must be on at least rack version `3.16.0` to use emptyDir volumes. Update your rack with the following command:

Ensure your rack is updated to version `3.16.0` or later. For detailed instructions on updating your rack, see the [Updating a Rack](https://docs.convox.com/management/cli-rack-management/) page.
