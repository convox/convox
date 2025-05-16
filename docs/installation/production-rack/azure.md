---
title: "Microsoft Azure"
draft: false
slug: Microsoft Azure
url: /installation/production-rack/azure
---
# Microsoft Azure
> Please note that these are instructions for installing a Rack via the command line. The easiest way to install a Rack is with the [Convox Web Console](https://console.convox.com)

## Initial Setup

### Azure CLI

- [Install the Azure CLI](https://docs.microsoft.com/en-us/cli/azure/install-azure-cli?view=azure-cli-latest)
- Run `az login`

### Terraform

- Install [Terraform](https://learn.hashicorp.com/terraform/getting-started/install.html)

### Convox CLI

- [Install the Convox CLI](/installation/cli)

## Environment

The following environment variables are required:

- `ARM_CLIENT_ID`
- `ARM_CLIENT_SECRET`
- `ARM_SUBSCRIPTION_ID`
- `ARM_TENANT_ID`

### Select Subscription
```html
    $ az account list
```
- `ARM_SUBSCRIPTION_ID` is the `id`
- `ARM_TENANT_ID` is the `tenantId`

### Create Service Principal
```html
    $ az account set --subscription="$ARM_SUBSCRIPTION_ID"
    $ az ad sp create-for-rbac --name=terraform --role=Owner --scopes="/subscriptions/$ARM_SUBSCRIPTION_ID"
```
- `ARM_CLIENT_ID` is the `appId`
- `ARM_CLIENT_SECRET` is the `password`

### Grant Permissions
```html
    # Grant Microsoft Graph API permissions using the new Microsoft Graph resource ID
    $ az ad app permission add --id $ARM_CLIENT_ID --api 00000003-0000-0000-c000-000000000000 --api-permissions e1fe6dd8-ba31-4d61-89e7-88639da4683d=Scope 19dbc75e-c2e2-444c-a770-ec69d8559fc7=Role
    
    # The minimum permission required is Directory.Read.All
    $ az ad app permission grant --id $ARM_CLIENT_ID --api 00000003-0000-0000-c000-000000000000 --consent-type AllPrincipals --scope Directory.Read.All
    
    # Perform admin consent for the permissions
    $ az ad app permission admin-consent --id $ARM_CLIENT_ID
```

> **Note**: For new Azure tenants, you must use Microsoft Graph API (not Azure AD Graph). The service principal must have either **Directory Reader role** or `Directory.Read.All` permission consented in Microsoft Graph.
## Install Rack
```html
    $ convox rack install azure <name> [param1=value1]...
```
### Available Parameters

| Name        | Default          | Description                                                             |
| ----------- | ---------------- | ----------------------------------------------------------------------- |
| **cert_duration**        | **2160h**          | Certification renew period                                                |
| **node_type**            | **Standard_D3_v3** | Node instance type                                                        |
| **region**               | **eastus**         | Azure region                                                              |
| **syslog**               |                    | Forward logs to a syslog endpoint (e.g. **tcp+tls://example.org:1234**)   |
