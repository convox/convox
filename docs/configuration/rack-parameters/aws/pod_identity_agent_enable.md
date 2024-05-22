---
title: "pod_identity_agent_enable"
draft: false
slug: pod_identity_agent_enable
url: /configuration/rack-parameters/aws/pod_identity_agent_enable
---

# pod_identity_agent_enable

## Description
The `pod_identity_agent_enable` parameter enables the AWS Pod Identity Agent. This allows Kubernetes pods to assume IAM roles, providing fine-grained access control for accessing AWS services.

## Default Value
The default value for `pod_identity_agent_enable` is `false`.

## Use Cases
- **Granular IAM Policies**: Assign specific IAM roles to pods, ensuring that each pod has only the permissions it needs.
- **Security Best Practices**: Avoid using static credentials inside pods and leverage IAM roles for security.

## Managing Parameters

### Viewing Current Parameters
```html
$ convox rack params -r rackName
pod_identity_agent_enable  false
```

### Setting Parameters
To enable the AWS Pod Identity Agent, use the following command:
```html
$ convox rack params set pod_identity_agent_enable=true -r rackName
Setting parameters... OK
```
This command enables the AWS Pod Identity Agent for your rack.

## Additional Information
Enabling the AWS Pod Identity Agent allows your applications running in Kubernetes to securely access AWS services by assuming IAM roles. This setup reduces the need for static AWS credentials within your application code. Ensure that you have configured IAM roles and Kubernetes service accounts properly to take full advantage of this feature.

### How to Use and Test
1. Enable the EKS pod identity feature by executing:
   ```html
   convox rack params set pod_identity_agent_enable=true -r rackName
   ```

2. Update your `convox.yml` to include the required AWS IAM policy ARNs:
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

3. Deploy your application changes using:
   ```html
   convox deploy -a appName -r rackName
   ```
   
By enabling this parameter, you enhance the security and manageability of your application's access to AWS resources.
