---
title: "Upgrading"
draft: false
slug: Upgrading
url: /help/upgrading
---
# Upgrading

## Local Racks

- To retain your local Apps when moving to the new local Rack, you should [export](/reference/cli/apps#apps-export) them all first.  This will create a local archive of all pertinent data for each app you export.
- Uninstall your old local rack: `convox rack uninstall local`.  This should be performed with your existing CLI before upgrading that.
- Deprecate your existing CLI version: `sudo mv /usr/local/bin/convox /usr/local/bin/convox-old`
- [Install](/installation/cli) the new CLI
- Install a new [local Rack](/installation/development-rack/)
- Create and then [Import](/reference/cli/apps#apps-import) your Apps from your previous export archives.

## ECS (Generation 2 Racks) -> EKS/GCP/Azure/Digital Ocean (Generation 3 Racks)

- To retain your Apps when moving to your new Kubernetes-based Rack, you should [export](/reference/cli/apps#apps-export) them all first.  This will create a local archive of all pertinent data for each app you export.
- Deprecate your existing CLI version: `sudo mv /usr/local/bin/convox /usr/local/bin/convox-old`
- [Install](/installation/cli) the new CLI
- [Install](/installation/production-rack/) a new Kubernetes-based Rack
- Create and then [Import](/reference/cli/apps#apps-import) your Apps from your previous exports.
- Once satisfied that your Apps are running successfully on your new Rack, you can redirect any DNS / custom Domains to your new apps.
- Then delete and remove your previous Apps and Rack.  This should be performed with the older version of the CLI. `convox-old apps delete <appname>` and `convox-old rack uninstall -r <rackname>` etc.
