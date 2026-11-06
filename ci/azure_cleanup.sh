#!/bin/bash
set -e

. "$(dirname "$0")/lib.sh"

# Set your azure access

az login --service-principal -u ${ARM_CLIENT_ID} -p ${ARM_CLIENT_SECRET} --tenant ${ARM_TENANT_ID}

az account set --subscription ${ARM_SUBSCRIPTION_ID}

clusters=$(az aks list --query "[?starts_with(name, 'ci')].{name: name, resourceGroup: resourceGroup}" -o tsv)

if [ -z "$clusters" ]; then
  echo "No ci AKS clusters to delete"
  exit 0
fi

while read -r name resourceGroup; do
  [ -z "$name" ] && continue
  ci_rack_is_old "$name" || { echo "skipping $name: younger than 8 hours"; continue; }
  echo "Deleting AKS cluster: $name in resource group $resourceGroup"
  az aks delete --name "$name" --resource-group "$resourceGroup" --yes --no-wait
done <<< "$clusters"
