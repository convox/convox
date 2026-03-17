---
title: "Upgrading"
slug: upgrading
url: /help/upgrading
---
# Upgrading

## Local Racks

To upgrade a local development Rack to a newer version:

```bash
convox rack update -r <RACK_NAME> <VERSION>
```

If you need to do a clean reinstall:

1. Export any apps you want to keep: `convox apps export <APP_NAME> -f app-backup.tgz`
2. Uninstall the old rack: `convox rack uninstall -r <RACK_NAME>`
3. Delete the minikube cluster: `minikube delete`
4. Follow the [development rack installation instructions](/installation/development-rack) to set up a fresh rack
5. Import your apps: `convox apps import <APP_NAME> -f app-backup.tgz`

## ECS (Generation 2 Racks) -> EKS/GCP/Azure/Digital Ocean (Generation 3 Racks)

- To retain your Apps when moving to your new Kubernetes-based Rack, you should [export](/reference/cli/apps#apps-export) them all first.  This will create a local archive of all pertinent data for each app you export.
- Deprecate your existing CLI version: `sudo mv /usr/local/bin/convox /usr/local/bin/convox-old`
- [Install](/installation/cli) the new CLI
- [Install](/installation/production-rack/) a new Kubernetes-based Rack
- Create and then [Import](/reference/cli/apps#apps-import) your Apps from your previous exports.
- Once satisfied that your Apps are running successfully on your new Rack, you can redirect any DNS / custom Domains to your new apps.
- Then delete and remove your previous Apps and Rack.  This should be performed with the older version of the CLI. `convox-old apps delete <appname>` and `convox-old rack uninstall -r <rackname>` etc.

## See Also

- [Changes from v2](/help/changes) for a detailed list of what changed between v2 and v3
