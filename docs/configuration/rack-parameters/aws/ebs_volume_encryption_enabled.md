---
title: "ebs_volume_encryption_enabled"
draft: false
slug: ebs_volume_encryption_enabled
url: /configuration/rack-parameters/aws/ebs_volume_encryption_enabled
---

# ebs_volume_encryption_enabled

## Description
The `ebs_volume_encryption_enabled` parameter enables encryption for EBS volumes used by the primary node disks in the rack's EKS cluster. When enabled, this feature provides an additional layer of security for data at rest on your Kubernetes worker nodes by encrypting all data stored on the EBS volumes using AWS-managed encryption keys.

## Default Value
The default value for `ebs_volume_encryption_enabled` is `false`.

## Use Cases
- **Enhanced Security**: Encrypts data at rest on EKS worker node disks, meeting compliance and security requirements.
- **Compliance Standards**: Helps meet regulatory requirements such as HIPAA, PCI DSS, or SOC 2 that mandate encryption of data at rest.
- **AWS Best Practices**: Aligns with AWS security recommendations for encrypting EBS volumes.
- **Zero-Trust Architecture**: Provides defense-in-depth security by ensuring all storage is encrypted by default.
- **Organizational Policies**: Meets internal security policies that require encryption of all persistent storage.

## Setting Parameters
To enable EBS volume encryption, use the following command:
```html
$ convox rack params set ebs_volume_encryption_enabled=true -r rackName
Setting parameters... OK
```

To disable EBS volume encryption:
```html
$ convox rack params set ebs_volume_encryption_enabled=false -r rackName
Setting parameters... OK
```

## Additional Information
- This parameter can be enabled on existing racks or configured during initial rack setup.
- When enabled on an existing rack, existing nodes will be converted to use encrypted EBS volumes during the next update cycle.
- New nodes will automatically be provisioned with encrypted EBS volumes once this parameter is enabled.
- The encryption uses AWS-managed encryption keys, providing security without impacting application performance.
- Once enabled, all primary node disks in the EKS cluster will have their EBS volumes encrypted.
- This setting affects only the primary node disks; other storage volumes (such as those used by applications) are managed separately.
- The encryption is transparent to applications and does not require any code changes.
- Disabling this parameter will provision new nodes without encryption, but existing encrypted volumes will remain encrypted.

## Security Considerations
- EBS volume encryption encrypts data at rest, providing protection against unauthorized access to the underlying storage.
- The encryption and decryption processes are handled automatically by AWS and do not require manual key management.
- This feature complements other security measures and should be part of a comprehensive security strategy.
- For additional security, consider implementing encryption in transit and application-level encryption as needed.

## Performance Impact
- EBS volume encryption has minimal performance impact on most workloads.
- The encryption/decryption operations are handled by the AWS infrastructure layer.
- No additional CPU or memory overhead is imposed on your applications.

## Version Requirements
This feature requires at least Convox rack version `3.21.5`.