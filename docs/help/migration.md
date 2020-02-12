# Migrating from previous Convox racks

## Local Racks

- To retain your local Apps when moving to the new local Rack, you should export them all first.
- Then uninstall your old local rack
- Uninstall your existing CLI version
- [Install](../installation/cli)) the new CLI
- Install a new local Rack
- Import your Apps from your previous exports.

## ECS -> EKS/GCP/Azure/Digital Ocean

- To retain your Apps when moving to your new Kubernetes-based Rack, you should export them all first.
- Uninstall your existing CLI version
- [Install](../installation/cli)) the new CLI
- Install a new Kubernetes-based Rack
- Import your Apps from your previous exports.
- Once satisfied that your Apps are running successfully on your new Rack, you can redirect any DNS / custom Domains to your new apps.
- Then delete and remove your previous Apps and Rack.
