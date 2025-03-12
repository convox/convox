---
title: "ecr_scan_on_push_enable"
draft: false
slug: ecr_scan_on_push_enable
url: /configuration/rack-parameters/aws/ecr_scan_on_push_enable
---

# ecr_scan_on_push_enable

## Description
The `ecr_scan_on_push_enable` parameter enables or disables the image scan feature for images pushed to an Amazon ECR repository. When enabled, each image pushed to the repository is automatically scanned for vulnerabilities.

This feature leverages Amazon ECR's native scanning capability, which identifies software vulnerabilities in your container images. After the scan completes, you can retrieve the scan results through the AWS Management Console or AWS CLI.

## Default Value
The default value for `ecr_scan_on_push_enable` is `false`.

## Use Cases
- **Security Compliance**: Automatically scan images for vulnerabilities upon pushing to the ECR repository, ensuring compliance with security policies.
- **Early Detection**: Detect potential security issues in container images early in the development process.
- **Vulnerability Management**: Identify and address vulnerabilities in your container images before deployment.
- **DevSecOps Integration**: Incorporate security scanning into your CI/CD pipeline.

## Setting Parameters
To enable automatic image scanning on push to ECR, use the following command:
```html
$ convox rack params set ecr_scan_on_push_enable=true -r rackName
Setting parameters... OK
```

When installing a new rack, you can include this parameter:
```html
$ convox rack install aws rackName ecr_scan_on_push_enable=true
```

To verify the parameter is set correctly:
```html
$ convox rack params -r rackName
```

## Additional Information
- When `ecr_scan_on_push_enable` is set to `true`, each image push to the ECR repository will trigger an automatic scan.
- Scan results are available through the AWS Management Console or AWS CLI.
- To view scan results via AWS CLI:
  ```bash
  $ aws ecr describe-image-scan-findings --repository-name your-repository-name --image-id imageTag=your-image-tag
  ```
- ECR uses the Common Vulnerabilities and Exposures (CVEs) database from the open-source Clair project to identify vulnerabilities.
- Scan results include details about identified vulnerabilities, including severity levels, descriptions, and CVE IDs.
- ECR image scanning helps identify vulnerabilities but does not automatically remediate them. Review scan results and take appropriate action based on your security requirements.
- There are no additional AWS charges for using ECR scan on push, but standard ECR usage costs apply.
- Only images pushed after enabling this feature will be automatically scanned. Existing images can be scanned manually through the AWS console or CLI.

## Version Requirements
This feature requires at least Convox rack version `3.18.7`.
