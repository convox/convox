---

title: "Microsoft Azure"
draft: false
slug: Microsoft Azure
url: /installation/production-rack/azure
----------------------------------------

# Microsoft Azure

> These are instructions for installing a Rack via the command line. The easiest way to install a Rack is with the [Convox Web Console](https://console.convox.com)

---

## Initial Setup

### 1 Install prerequisites

| Tool                      | Docs                                                                                                             |
| ------------------------- | ---------------------------------------------------------------------------------------------------------------- |
| **Azure CLI**             | [https://docs.microsoft.com/cli/azure/install-azure-cli](https://docs.microsoft.com/cli/azure/install-azure-cli) |
| **Terraform** (optional)  | [https://developer.hashicorp.com/terraform/tutorials](https://developer.hashicorp.com/terraform/tutorials)       |
| **Convox CLI**            | [/installation/cli](/installation/cli)                                                                           |

```bash
# sign‑in to Azure
az login
```

---

## 2 Environment variables required by Terraform

| Variable              | Where to get it                                     |
| --------------------- | --------------------------------------------------- |
| `ARM_CLIENT_ID`       | **appId** of the service‑principal you create below |
| `ARM_CLIENT_SECRET`   | **password** returned when the SP is created        |
| `ARM_SUBSCRIPTION_ID` | `az account show --query id -o tsv`                 |
| `ARM_TENANT_ID`       | `az account show --query tenantId -o tsv`           |

---

## 3 Select your subscription

```bash
az account list --output table
az account set --subscription <SUBSCRIPTION_ID>
```

---

## 4 Create the Terraform service‑principal

```bash
az ad sp create-for-rbac \
  --name terraform \
  --role Owner \
  --scopes "/subscriptions/$ARM_SUBSCRIPTION_ID"
```

* `ARM_CLIENT_ID` → `appId`
* `ARM_CLIENT_SECRET` → `password`

---

## 5 Grant Microsoft Graph permissions (API scope)

```bash
az ad app permission add --id $ARM_CLIENT_ID \
  --api 00000003-0000-0000-c000-000000000000 \
  --api-permissions 311a71cc-e848-46a1-bdf8-97ff7156d8e6=Scope

az ad app permission grant --id $ARM_CLIENT_ID \
  --api 00000003-0000-0000-c000-000000000000 \
  --consent-type AllPrincipals --scope User.Read

az ad app permission admin-consent --id $ARM_CLIENT_ID
```

---

## 6 **NEW — Assign the *********************Application Administrator********************* directory role**

Terraform/Convox must be able to create Azure AD (Entra) application objects. Because Azure RBAC roles do **not** flow into Entra ID, you need to grant the service‑principal a **directory role**.

> The quickest safe choice is **Application Administrator**. (Cloud Application Administrator also works.)

### 6·1 Assign via Azure CLI + Microsoft Graph

```bash
# 1) Get the internal roleDefinitionId for "Application Administrator"
ROLE_ID=$(az rest --method GET \
  --url "https://graph.microsoft.com/v1.0/roleManagement/directory/roleDefinitions?\$filter=displayName eq 'Application Administrator'" \
  --query 'value[0].id' -o tsv)

# 2) Get the objectId of the SP you just created (replace display‑name if different)
SP_OBJECT_ID=$(az ad sp list --display-name "terraform" --query "[0].id" -o tsv)

# 3) Assign the role at tenant scope (\"/\")
az rest --method POST \
  --url "https://graph.microsoft.com/v1.0/roleManagement/directory/roleAssignments" \
  --body "{\n    \"principalId\": \"${SP_OBJECT_ID}\",\n    \"roleDefinitionId\": \"${ROLE_ID}\",\n    \"directoryScopeId\": \"/\"\n  }"
```

---

## 7 Install the Rack

```bash
convox rack install azure <name> [param=value]…
```

| Parameter       | Default          | Description                       |
| --------------- | ---------------- | --------------------------------- |
| `cert_duration` | `2160h`          | How often certificates renew      |
| `node_type`     | `Standard_D3_v3` | VM size for Kubernetes nodes      |
| `region`        | `eastus`         | Azure region                      |
| `syslog`        |                  | Forward logs to a syslog endpoint |

---

## 8 Troubleshooting “Insufficient privileges” (403) errors

If `terraform apply` fails with:

```
ApplicationsClient.BaseClient.Post(): unexpected status 403 … Authorization_RequestDenied: Insufficient privileges to complete the operation.
```

verify that **step 6** (Application Administrator role) succeeded.
