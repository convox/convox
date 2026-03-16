---
title: "Production Rack"
slug: production-rack
url: /installation/production-rack
---
# Production Rack

A production Rack runs on a cloud provider and hosts your applications in a real Kubernetes cluster. Each provider has specific prerequisites (cloud account, IAM permissions, region selection) covered in its installation guide. Production Racks support multi-app deployments, autoscaling, managed resources, and all [Rack Parameters](/configuration/rack-parameters) for your provider.

> We recommend installing production Racks using the [Convox Web Console](https://console.convox.com). Console installation handles Terraform state management and makes it easier to manage rack updates, parameters, and access control. However you can install a production Rack via the command line using the instructions for each specific cloud provider linked here. If you install via the command line and you later want to manage that Rack from the Web Console you will need to [move your Rack](/management/console-rack-management) into the Web Console first.

## Select a Provider

- [Amazon Web Services](/installation/production-rack/aws)
- [Digital Ocean](/installation/production-rack/do)
- [Google Cloud](/installation/production-rack/gcp)
- [Microsoft Azure](/installation/production-rack/azure)

## See Also

- [CLI Rack Management](/management/cli-rack-management) for managing racks after installation
- [Rack Parameters](/configuration/rack-parameters) for customizing your rack configuration
