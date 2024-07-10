---
title: "ecr_scan_on_push_enable"
draft: false
slug: ecr_scan_on_push_enable
url: /configuration/rack-parameters/aws/ecr_scan_on_push_enable
---

# ecr_scan_on_push_enable

## Description
The `ecr_scan_on_push_enable` parameter enables automatic security scanning of container images pushed to your ECR repositories.

## Default Value
The default value for `ecr_scan_on_push_enable` is `false`.

## Use Cases
- **Improved Security Posture**: Automatically scan container images for vulnerabilities helps identify potential security risks before deploying them to production.
- **Streamlined Workflow**: Automating vulnerability scans saves time and ensures consistent security practices.

## Setting Parameters
To enable the ECR Scan on Push feature, use the following command:
```html
$ convox rack params set ecr_scan_on_push_enable=true -r rackName
Setting parameters... OK
```
This command will trigger vulnerability scans for any new image pushed to ECR repositories associated with your rack.

## Additional Information
Enabling ecr_scan_on_push_enable provides an extra layer of security for your containerized applications. When enabled, It will trigger a vulnerability scan whenever a new image is pushed to a repository within your rack.

### How to Use
1. Enable automatic image scanning on push by executing:
   ```html
   convox rack params set ecr_scan_on_push_enable=true -r rackName
   ```

2. Push your container image to the ECR repository within your rack. It will automatically trigger a vulnerability scan.
   
By enabling this parameter, you can automate the initial security assessment of your container images.
