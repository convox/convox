---
title: "Cloud CLI Reference"
description: "The convox cloud command namespace manages machines and deploys apps to Convox Cloud, with every command following the convox cloud subcommand pattern."
slug: cli-reference
url: /cloud/cli-reference
---

# Cloud CLI Reference

The `convox cloud` command namespace provides all the tools needed to manage machines and deploy applications to Convox Cloud. All cloud commands follow the pattern `convox cloud <command>`.

## Global Options

All `convox cloud` commands support these global options:

| Option | Alias | Description |
|--------|-------|-------------|
| `--app <name>` | `-a` | Specify the application |
| `--machine <name>` | `-i` | Specify the machine (required for most commands) |
| `--config <name>` | | Specify the config to use |

## Command Groups

The cloud commands are organized into the following groups. Select a group to see its commands, flags, examples, and output.

- [Machine Management](/cloud/cli-reference/machines) - List machines in your organization.
- [Application Commands](/cloud/cli-reference/apps) - List, create, delete, export, import, and inspect apps and their parameters.
- [Build Commands](/cloud/cli-reference/builds) - Create, list, export, import, inspect builds and view build logs.
- [Deployment Commands](/cloud/cli-reference/deploy) - Build and promote in a single command.
- [Environment Commands](/cloud/cli-reference/env) - List, edit, get, set, and unset environment variables.
- [Process Management](/cloud/cli-reference/processes) - Exec into, list, stop, and run one-off processes.
- [Release Management](/cloud/cli-reference/releases) - List, create, inspect, promote, and rollback releases.
- [Service Management](/cloud/cli-reference/services) - List, restart, and scale services.
- [Resource Management (Cloud Databases)](/cloud/cli-reference/resources) - Interact with managed Cloud Databases.
- [Monitoring Commands](/cloud/cli-reference/logs) - Stream and filter application logs.
- [Configuration Management](/cloud/cli-reference/config) - Get, set, and list configurations.
- [Utility Commands](/cloud/cli-reference/utility) - Copy files to and from processes, and run test suites.

## Common Patterns

### Complete Deployment Workflow

```bash
# Create machine via Console first (see Machine Management section)
# Then deploy application
$ convox cloud deploy -i prod --description "Initial deployment"

# Set environment variables
$ convox cloud env set DATABASE_URL=postgres://... -a myapp -i prod

# Scale service
$ convox cloud scale web --count 3 --cpu 500 -a myapp -i prod

# Monitor logs (streams by default; use --no-follow for one-time dump)
$ convox cloud logs -a myapp -i prod
```

### Development Workflow

```bash
# Create dev machine via Console
# Then deploy
$ convox cloud deploy -i dev

# Run migrations
$ convox cloud run web "rake db:migrate" -a myapp -i dev

# Debug with shell
$ convox cloud run web bash -a myapp -i dev
```

### Backup and Restore

```bash
# Export application
$ convox cloud apps export -a myapp -i prod --file backup.tgz

# Export database
$ convox cloud resources export database -a myapp -i prod --file db.sql

# Import to new machine
$ convox cloud apps import -a myapp -i staging --file backup.tgz
$ convox cloud resources import database -a myapp -i staging --file db.sql
```

## Tips and Best Practices

1. **Always specify the machine**: Most commands require `-i <machine>`
2. **Use aliases**: Set up shell aliases for common commands
3. **Watch deployments**: Use `--watch` flags to monitor progress
4. **Describe changes**: Use `--description` for builds and deploys
5. **Test in staging**: Deploy to a staging machine before production

## Error Handling

Common error patterns and solutions:

| Error | Solution |
|-------|----------|
| "machine not found" | Verify machine name with `convox cloud machines` |
| "insufficient resources" | Scale down services or upgrade machine size |
| "build timeout" | Optimize Dockerfile or contact support |
| "permission denied" | Ensure you're logged in: `convox login` |

## Getting Help

For detailed help on any command:

```bash
$ convox cloud <command> --help
```

For support:
- Documentation: [Convox Docs](/getting-started/introduction)
- Community: [community.convox.com](https://community.convox.com)
- Email: cloud-support@convox.com
