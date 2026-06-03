#!/bin/bash
set -e

# install az
sudo apt-get update

curl -sL https://aka.ms/InstallAzureCLIDeb | sudo bash


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
  echo "Deleting AKS cluster: $name in resource group $resourceGroup"
  az aks delete --name "$name" --resource-group "$resourceGroup" --yes --no-wait
done <<< "$clusters"
