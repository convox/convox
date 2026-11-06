#!/usr/bin/env bash
# bootstrap-terraform-sp.sh
# Creates a Terraform service‑principal, grants it Application Administrator,
# and prints the four ARM_* variables you need.

set -euo pipefail

# ----- config -----
SP_NAME="terraform"

# ----- subscription / tenant -----
SUBSCRIPTION_ID=$(az account show --query id -o tsv)
TENANT_ID=$(az account show --query tenantId -o tsv)

echo "SUBSCRIPTION_ID=$SUBSCRIPTION_ID"
echo "TENANT_ID=$TENANT_ID"

echo "
Creating service principal '$SP_NAME'…"
SP_JSON=$(az ad sp create-for-rbac --name "$SP_NAME" \
  --role Owner \
  --scopes "/subscriptions/$SUBSCRIPTION_ID" \
  --query "{appId: appId, password: password}" -o json)

# Extract values safely from the clean JSON payload
CLIENT_ID=$(echo "$SP_JSON" | jq -r .appId)
CLIENT_SECRET=$(echo "$SP_JSON" | jq -r .password)

echo "CLIENT_ID=$CLIENT_ID"
echo "CLIENT_SECRET=$CLIENT_SECRET"

# ----- grant Application Administrator directory role -----
echo "\nGranting Application Administrator directory role…"
ROLE_ID=$(az rest --method GET \
  --url "https://graph.microsoft.com/v1.0/roleManagement/directory/roleDefinitions?\$filter=displayName eq 'Application Administrator'" \
  --query 'value[0].id' -o tsv)

SP_OBJECT_ID=$(az ad sp list --display-name "$SP_NAME" --query "[0].id" -o tsv)

az rest --method POST \
  --url "https://graph.microsoft.com/v1.0/roleManagement/directory/roleAssignments" \
  --body "{ \"principalId\": \"${SP_OBJECT_ID}\", \"roleDefinitionId\": \"${ROLE_ID}\", \"directoryScopeId\": \"/\" }" \
  --output none

echo "\n✓ Service‑principal prepped ✔"

echo "
# ---- copy these values into the Convox Web Console ----"
echo "ARM_CLIENT_ID=$CLIENT_ID"
echo "ARM_CLIENT_SECRET=$CLIENT_SECRET"
echo "ARM_SUBSCRIPTION_ID=$SUBSCRIPTION_ID"
echo "ARM_TENANT_ID=$TENANT_ID"
