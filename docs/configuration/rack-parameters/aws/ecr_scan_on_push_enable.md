---
title: "ecr_scan_on_push_enable"
draft: false
slug: ecr_scan_on_push_enable
url: /configuration/rack-parameters/aws/ecr_scan_on_push_enable
---

# ecr_scan_on_push_enable

## Description
The `ecr_scan_on_push_enable` parameter enables or disables the image scan feature for images pushed to an Amazon ECR repository. When enabled, each image pushed to the repository is automatically scanned for vulnerabilities.

## Default Value
The default value for `ecr_scan_on_push_enable` is `false`.

## Use Cases
- **Security Compliance**: Automatically scan images for vulnerabilities upon pushing to the ECR repository, ensuring compliance with security policies.
- **Early Detection**: Detect potential security issues in container images early in the development process.

## Setting Parameters
To enable the `ecr_scan_on_push_enable` parameter, use the following command:
```html
$ convox rack params set ecr_scan_on_push_enable=true -r rackName
Setting parameters... OK
```
This command enables the image scan feature for all images pushed to the ECR repository.

## Additional Information
Enabling `ecr_scan_on_push_enable` helps in maintaining a secure container environment by ensuring that all images are scanned for known vulnerabilities upon being pushed to the repository. This feature leverages Amazon ECR's image scanning capabilities to provide detailed reports on security findings, aiding in proactive vulnerability management.

When `ecr_scan_on_push_enable` is set to `true`, each image push to the ECR repository will trigger an automatic scan, with results accessible through the AWS Management Console or AWS CLI. This setting helps in identifying and mitigating security risks in the container images before they are deployed in production environments.
