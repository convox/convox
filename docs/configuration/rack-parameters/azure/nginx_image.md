---
title: "nginx_image"
slug: nginx_image
url: /configuration/rack-parameters/azure/nginx_image
---

# nginx_image

## Description
The `nginx_image` parameter allows you to specify a custom container image for the nginx ingress controller. This is useful for using a specific version or a custom-built nginx image.

## Default Value
The default value is an empty string (`""`), which uses the built-in default image: `registry.k8s.io/ingress-nginx/controller:v1.12.0`.

## Use Cases
- **Version Pinning**: Lock to a specific nginx ingress controller version for stability.
- **Custom Builds**: Use a custom nginx image with additional modules or patches.
- **Air-Gapped Environments**: Point to an image in a private registry.

## Setting Parameters
To set the `nginx_image` parameter, use the following command:
```bash
$ convox rack params set nginx_image=registry.k8s.io/ingress-nginx/controller:v1.12.0 -r rackName
Setting parameters... OK
```

## Additional Information
The image must be a valid nginx ingress controller image compatible with the Kubernetes ingress-nginx project. Using an incompatible image may cause routing failures.
