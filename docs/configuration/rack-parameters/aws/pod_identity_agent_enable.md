---
title: "pod_identity_agent_enable"
draft: false
slug: pod_identity_agent_enable
url: /configuration/rack-parameters/aws/pod_identity_agent_enable
---

# pod_identity_agent_enable

## Description
The `pod_identity_agent_enable` parameter enables the AWS Pod Identity Agent in your EKS cluster. This feature allows Kubernetes pods to assume IAM roles directly, providing a more secure way to grant AWS permissions to your applications without using long-lived credentials or environment variables.

When enabled, this parameter installs and configures the AWS Pod Identity Agent, which facilitates the association between Kubernetes service accounts and AWS IAM roles.

## Default Value
The default value for `pod_identity_agent_enable` is `false`.

## Use Cases
- **Enhanced Security**: Replace AWS access keys with IAM roles for pods, reducing the risk associated with long-lived credentials.
- **Fine-grained Access Control**: Apply precise IAM policies to specific services or components within your application.
- **Regulatory Compliance**: Meet security requirements by implementing the principle of least privilege for AWS resource access.
- **Simplified Credential Management**: Eliminate the need to manage and rotate AWS credentials within your applications.

## Setting Parameters
To enable the AWS Pod Identity Agent, use the following command:
```html
$ convox rack params set pod_identity_agent_enable=true -r rackName
Setting parameters... OK
```

## Using with Applications
Once enabled at the rack level, you can configure services in your `convox.yml` file to use specific IAM policies:

```yaml
services:
  web:
    build: .
    port: 3000
    accessControl:
      awsPodIdentity:
        policyArns:
          - "arn:aws:iam::aws:policy/AmazonEC2ContainerRegistryReadOnly"
          - "arn:aws:iam::aws:policy/AmazonS3ReadOnlyAccess"
          - "arn:aws:iam::123456789012:policy/MyCustomPolicy"
```

In this example, the `web` service is granted permissions defined in three AWS IAM policies:
1. Read-only access to ECR repositories
2. Read-only access to S3 buckets
3. A custom policy you've defined in your AWS account

## Additional Information
- The Pod Identity Agent creates a new IAM role for each service that uses the `awsPodIdentity` configuration.
- The IAM role names follow the pattern `eksctl-<cluster-name>-addon-pod-identity-role`.
- Applications using Pod Identity don't need to include AWS credentials in their environment variables.
- When a pod is created, the identity agent automatically injects AWS credentials into the pod's environment.
- This implementation leverages EKS Pod Identity, which is the AWS-recommended approach for pod IAM access.
- Pod Identity replaces the older kiam/kube2iam pattern as well as the IAM Roles for Service Accounts (IRSA) approach.
- Each service can have different IAM policies attached, allowing for precise access control.

## Version Requirements
This feature requires at least Convox rack version `3.18.1`.
