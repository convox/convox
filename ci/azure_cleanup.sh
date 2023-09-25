#!/bin/bash

# install az
sudo apt-get update

curl -sL https://aka.ms/InstallAzureCLIDeb | sudo bash


# Set your azure access

az login --service-principal -u ${ARM_CLIENT_ID} -p ${ARM_CLIENT_SECRET} --tenant ${ARM_TENANT_ID}

clusters=$(az aks list --query "[?starts_with(name, 'ci')].{name: name, resourceGroup: resourceGroup}" -o tsv)

echo "$clusters" | while read -r name resourceGroup; do
  echo "Deleting AKS cluster: $name in resource group $resourceGroup"
  az aks delete --name "$name" --resource-group "$resourceGroup" --yes --no-wait
done
