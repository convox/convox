# Microsoft Azure

## Initial Setup

### Azure CLI

- [Install the Azure CLI](https://docs.microsoft.com/en-us/cli/azure/install-azure-cli?view=azure-cli-latest)
- Run `az login`

### Convox CLI

- [Install the Convox CLI](../cli.md)

## Environment

The following environment variables are required:

- `ARM_CLIENT_ID`
- `ARM_CLIENT_SECRET`
- `ARM_SUBSCRIPTION_ID`
- `ARM_TENANT_ID`

### Select Subscription

    $ az account list

- `ARM_SUBSCRIPTION_ID` is the `id`
- `ARM_TENANT_ID` is the `tenantId`

### Create Service Principal

    $ az account set --subscription="$ARM_SUBSCRIPTION_ID"
    $ az ad sp create-for-rbac --name=terraform --role=Owner --scopes="/subscriptions/$ARM_SUBSCRIPTION_ID"

- `ARM_CLIENT_ID` is the `appId`
- `ARM_CLIENT_SECRET` is the `password`

### Grant Permissions

    $ az ad app permission add --id $ARM_CLIENT_ID --api 00000002-0000-0000-c000-000000000000 --api-permissions 311a71cc-e848-46a1-bdf8-97ff7156d8e6=Scope 824c81eb-e3f8-4ee6-8f6d-de7f50d565b7=Role
    $ az ad app permission grant --id $ARM_CLIENT_ID --api 1489bf2e-4a8b-43c5-a319-8eda31218f4c --consent-type AllPrincipals --scope User.Read
    $ az ad app permission admin-consent --id $ARM_CLIENT_ID 

## Install Rack

    $ convox rack install azure <name>