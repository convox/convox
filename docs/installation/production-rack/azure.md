---

title: "Microsoft Azure"
draft: false
slug: Microsoft Azure
url: /installation/production-rack/azure
----------------------------------------

# Microsoft Azure

> These are instructions for installing a Rack via the command line. The easiest way to install a Rack is with the [Convox Web Console](https://console.convox.com)

---

## Initial Setup

### 1 Install prerequisites

| Tool                      | Docs                                                                                                             |
| ------------------------- | ---------------------------------------------------------------------------------------------------------------- |
| **Azure CLI**             | [https://docs.microsoft.com/cli/azure/install-azure-cli](https://docs.microsoft.com/cli/azure/install-azure-cli) |
| **Terraform** (optional)  | [https://developer.hashicorp.com/terraform/tutorials](https://developer.hashicorp.com/terraform/tutorials)       |
| **Convox CLI**            | [/installation/cli](/installation/cli)                                                                           |

```bash
# sign‑in to Azure
az login
```

**Expected output:**
```
A web browser has been opened at https://login.microsoftonline.com/organizations/oauth2/v2.0/authorize. 
Please continue the login in the web browser.

[Tenant and subscription selection]

No     Subscription name    Subscription ID                       Tenant
-----  -------------------  ------------------------------------  --------
[1] *  Your Subscription    xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx  YourTenant

The default is marked with an *; the default tenant is 'YourTenant' and subscription is 'Your Subscription'.
```

---

## 2 Environment variables required by Terraform

| Variable              | Where to get it                                     |
| --------------------- | --------------------------------------------------- |
| `ARM_CLIENT_ID`       | **appId** of the service‑principal you create below |
| `ARM_CLIENT_SECRET`   | **password** returned when the SP is created        |
| `ARM_SUBSCRIPTION_ID` | `az account show --query id -o tsv`                 |
| `ARM_TENANT_ID`       | `az account show --query tenantId -o tsv`           |

---

## 3 Select your subscription and set environment variables

```bash
# List available subscriptions
az account list --output table

# Set your subscription (replace with your actual subscription ID)
az account set --subscription <SUBSCRIPTION_ID>

# Export the subscription ID for later use
export ARM_SUBSCRIPTION_ID=$(az account show --query id -o tsv)

# Verify the subscription ID is set
echo "Subscription ID: $ARM_SUBSCRIPTION_ID"
```

**Expected output:**
```bash
# az account list --output table
Name             CloudName    SubscriptionId                        State    IsDefault
---------------  -----------  ------------------------------------  -------  -----------
Your Subscription AzureCloud   xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx  Enabled  True

# echo "Subscription ID: $ARM_SUBSCRIPTION_ID"
Subscription ID: xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
```

---

## 4 Create the Terraform service‑principal

```bash
az ad sp create-for-rbac \
  --name terraform \
  --role Owner \
  --scopes "/subscriptions/$ARM_SUBSCRIPTION_ID"
```

**Expected output:**
```json
{
  "appId": "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx",
  "displayName": "terraform",
  "password": "xxxxx~xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
  "tenant": "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"
}
```

**Set environment variables from the output:**
```bash
# Copy the appId from the output above
export ARM_CLIENT_ID="xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"

# Copy the password from the output above  
export ARM_CLIENT_SECRET="xxxxx~xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"

# Copy the tenant from the output above
export ARM_TENANT_ID="xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"
```

---

## 5 Grant Microsoft Graph permissions (API scope)

**Note:** The original documentation had an incorrect permission ID. Use the corrected commands below:

```bash
# Add User.Read delegated permission (corrected permission ID)
az ad app permission add --id $ARM_CLIENT_ID \
  --api 00000003-0000-0000-c000-000000000000 \
  --api-permissions e1fe6dd8-ba31-4d61-89e7-88639da4683d=Scope

# Grant the permission
az ad app permission grant --id $ARM_CLIENT_ID \
  --api 00000003-0000-0000-c000-000000000000 \
  --consent-type AllPrincipals --scope User.Read

# Provide admin consent
az ad app permission admin-consent --id $ARM_CLIENT_ID
```

**Expected output:**
```bash
# First command output:
Invoking `az ad app permission grant --id xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx --api 00000003-0000-0000-c000-000000000000` is needed to make the change effective

# Second command output:
{
  "@odata.context": "https://graph.microsoft.com/v1.0/$metadata#oauth2PermissionGrants/$entity",
  "clientId": "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx",
  "consentType": "AllPrincipals",
  "id": "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
  "principalId": null,
  "resourceId": "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx",
  "scope": "User.Read"
}

# Third command completes silently if successful
```

---

## 6 **NEW — Assign the Application Administrator directory role**

Terraform/Convox must be able to create Azure AD (Entra) application objects. Because Azure RBAC roles do **not** flow into Entra ID, you need to grant the service‑principal a **directory role**.

> The quickest safe choice is **Application Administrator**. (Cloud Application Administrator also works.)

### 6·1 Assign via Azure CLI + Microsoft Graph

```bash
# 1) Get the internal roleDefinitionId for "Application Administrator"
ROLE_ID=$(az rest --method GET \
  --url "https://graph.microsoft.com/v1.0/roleManagement/directory/roleDefinitions?\$filter=displayName eq 'Application Administrator'" \
  --query 'value[0].id' -o tsv)

# Verify the role ID was retrieved
echo "Role ID: $ROLE_ID"

# 2) Get the objectId of the SP you just created
SP_OBJECT_ID=$(az ad sp list --display-name "terraform" --query "[0].id" -o tsv)

# Verify the service principal object ID was retrieved
echo "Service Principal Object ID: $SP_OBJECT_ID"

# 3) Assign the role at tenant scope ("/") - CORRECTED with Content-Type header
az rest --method POST \
  --url "https://graph.microsoft.com/v1.0/roleManagement/directory/roleAssignments" \
  --headers "Content-Type=application/json" \
  --body "{\"principalId\": \"${SP_OBJECT_ID}\", \"roleDefinitionId\": \"${ROLE_ID}\", \"directoryScopeId\": \"/\"}"
```

**Expected output:**
```bash
# echo "Role ID: $ROLE_ID"
Role ID: 9b895d92-2cd3-44c7-9d02-a6ac2d5ea5c3

# echo "Service Principal Object ID: $SP_OBJECT_ID"  
Service Principal Object ID: xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx

# Role assignment command output:
{
  "@odata.context": "https://graph.microsoft.com/v1.0/$metadata#roleManagement/directory/roleAssignments/$entity",
  "directoryScopeId": "/",
  "id": "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
  "principalId": "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx",
  "roleDefinitionId": "9b895d92-2cd3-44c7-9d02-a6ac2d5ea5c3"
}
```

---

## 7 Verify your environment variables

Before proceeding to install the Rack, verify all required environment variables are set:

```bash
echo "ARM_SUBSCRIPTION_ID: $ARM_SUBSCRIPTION_ID"
echo "ARM_CLIENT_ID: $ARM_CLIENT_ID"  
echo "ARM_CLIENT_SECRET: $ARM_CLIENT_SECRET"
echo "ARM_TENANT_ID: $ARM_TENANT_ID"
```

**Expected output:**
```bash
ARM_SUBSCRIPTION_ID: xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
ARM_CLIENT_ID: xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
ARM_CLIENT_SECRET: xxxxx~xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
ARM_TENANT_ID: xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
```

**Important:** If any of these are empty, go back and re-run the relevant commands from steps 3-4.

---

## 8 Install the Rack

```bash
convox rack install azure <name> [param=value]…
```

| Parameter       | Default          | Description                       |
| --------------- | ---------------- | --------------------------------- |
| `cert_duration` | `2160h`          | How often certificates renew      |
| `node_type`     | `Standard_D3_v3` | VM size for Kubernetes nodes      |
| `region`        | `eastus`         | Azure region                      |
| `syslog`        |                  | Forward logs to a syslog endpoint |

**Example:**
```bash
convox rack install azure my-rack region=westus2 node_type=Standard_D2_v3
```

---

## 9 Troubleshooting

### Common Issues and Solutions

#### Issue: "The Required Resource Access specified in the request is invalid"
**Cause:** Using incorrect permission ID in step 5.
**Solution:** Ensure you're using the corrected permission ID: `e1fe6dd8-ba31-4d61-89e7-88639da4683d`

#### Issue: "Write requests must contain the Content-Type header declaration"
**Cause:** Missing Content-Type header in the role assignment REST call.
**Solution:** Ensure you include `--headers "Content-Type=application/json"` in the az rest command.

#### Issue: "Insufficient privileges" (403) errors during Terraform apply
**Cause:** Application Administrator role assignment failed or incomplete.
**Solution:** 
1. Verify step 6 completed successfully
2. Check that the service principal has the role assigned:
   ```bash
   az rest --method GET \
     --url "https://graph.microsoft.com/v1.0/roleManagement/directory/roleAssignments?\$filter=principalId eq '${SP_OBJECT_ID}'"
   ```

#### Issue: Environment variables not persisting
**Cause:** Variables only exist in current shell session.
**Solution:** Either re-export them or add them to your shell profile:
```bash
# Add to ~/.bashrc or ~/.zshrc
export ARM_SUBSCRIPTION_ID="your-subscription-id"
export ARM_CLIENT_ID="your-client-id"
export ARM_CLIENT_SECRET="your-client-secret"  
export ARM_TENANT_ID="your-tenant-id"
```

### Verification Commands

To verify your setup is correct before installing:

```bash
# Test Azure CLI authentication
az account show

# Test service principal can access subscription
az login --service-principal -u $ARM_CLIENT_ID -p $ARM_CLIENT_SECRET --tenant $ARM_TENANT_ID
az account show

# Switch back to your user account
az login
```

---

## 10 Alternative: Using Convox Console Runtime Integration

Instead of installing the Rack via CLI, you can use the **Convox Console** to create and manage your Azure integration. This provides a web-based interface for Rack management.

### 10·1 Navigate to Convox Console

1. Go to [https://console.convox.com/](https://console.convox.com/)
2. Sign in to your Convox account
3. Navigate to the **Integrations** page
4. Select **Install Azure Runtime**

### 10·2 Gather Required Information

You'll need the following values from the previous steps. Here's where to find each one:

| Console Field | Source | Example Value |
|---------------|--------|---------------|
| **Subscription ID** | From `az login` output or `$ARM_SUBSCRIPTION_ID` | `12345678-1234-1234-1234-123456789abc` |
| **Tenant ID** | From `az login` output or service principal `tenant` field | `87654321-4321-4321-4321-cba987654321` |
| **Client ID** | Service principal `appId` field | `abcdef12-3456-7890-abcd-ef1234567890` |
| **Client Secret** | Service principal `password` field | `ABC8Q~X12DeF34gH56iJ78kL90mN-OpQr23StUv` |

### 10·3 Mapping Service Principal Output to Console Fields

From your service principal creation output:

```json
{
  "appId": "abcdef12-3456-7890-abcd-ef1234567890",      ← Client ID
  "displayName": "terraform",
  "password": "ABC8Q~X12DeF34gH56iJ78kL90mN-OpQr23StUv", ← Client Secret  
  "tenant": "87654321-4321-4321-4321-cba987654321"       ← Tenant ID
}
```

**Quick reference commands to get these values:**
```bash
# Subscription ID
echo "Subscription ID: $ARM_SUBSCRIPTION_ID"

# Or get it directly
az account show --query id -o tsv

# Tenant ID  
az account show --query tenantId -o tsv

# Client ID and Secret (from your service principal output above)
echo "Client ID: $ARM_CLIENT_ID"
echo "Client Secret: $ARM_CLIENT_SECRET"
```

### 10·4 Complete the Integration

1. **Fill in the form** with the values from above
2. **Click "Install" or "Create Integration"**
3. **Wait for validation** - the Console will test the connection
4. **Create your Rack** through the Console interface

### 10·5 Benefits of Console Integration

- **Visual interface** for Rack management
- **Automatic updates** and maintenance
- **Monitoring and logging** built-in
- **Team collaboration** features
- **Easier troubleshooting** with visual status indicators

**Note:** You still need to complete steps 1-6 from this documentation to create the properly configured service principal, regardless of whether you use CLI or Console installation.