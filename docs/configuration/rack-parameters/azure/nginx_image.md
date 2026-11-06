---
title: "nginx_image"
description: "The nginx_image Azure rack parameter sets a custom container image for the nginx ingress controller, defaulting to the built-in ingress-nginx controller image."
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
The `nginx_image` parameter is managed internally by the Rack and is normally updated through `convox rack update`. Setting it directly without the `--force` flag returns an error:
```bash
$ convox rack params set nginx_image=registry.k8s.io/ingress-nginx/controller:v1.12.0 -r rackName
ERROR: param 'nginx_image' is managed internally — to update it use 'convox rack update'. Use --force to override
```

To set the parameter directly, pass the `--force` flag. The CLI still prints a warning that setting a managed parameter directly may break your Rack:
```bash
$ convox rack params set nginx_image=registry.k8s.io/ingress-nginx/controller:v1.12.0 --force -r rackName
Updating parameters... OK
```

## Additional Information
The image must be a valid nginx ingress controller image compatible with the Kubernetes ingress-nginx project. Using an incompatible image may cause routing failures.
