---
title: "Cloud CLI Reference"
draft: false
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

## Machine Management

### machines

List all machines in your organization.

```bash
$ convox cloud machines
NAME         SIZE    REGION      STATUS   CREATED
production   large   us-east-1   running  2 weeks ago
staging      small   us-west-2   running  1 month ago
```

**Note**: To create, update, or delete machines, use the Convox Console. Log in at [console.convox.com](https://console.convox.com) and navigate to the Cloud Machines page.

## Application Commands

### apps

List applications on a machine.

```bash
$ convox cloud apps -i <machine>
APP       STATUS   RELEASE
web-app   running  RABCDEFGHI
api       running  RBCDEFGHIJ
```

### apps cancel

Cancel an in-progress app update.

```bash
$ convox cloud apps cancel [app] -i <machine>
```

**Example:**
```bash
$ convox cloud apps cancel -a myapp -i production
Cancelling deployment of myapp... OK
```

### apps create

Create a new application on a machine.

```bash
$ convox cloud apps create [name] -i <machine>
```

**Options:**
- `--generation`: App generation (defaults to 3)
- `--timeout`: Creation timeout

**Example:**
```bash
$ convox cloud apps create myapp -i production
Creating myapp... OK
```

### apps delete

Delete an application from a machine.

```bash
$ convox cloud apps delete <app> -i <machine>
```

**Example:**
```bash
$ convox cloud apps delete oldapp -i production
Deleting oldapp... OK
```

### apps export

Export an application configuration and data.

```bash
$ convox cloud apps export [app] -i <machine>
```

**Options:**
- `--file`: Output file path

**Example:**
```bash
$ convox cloud apps export -a myapp -i production --file myapp-backup.tgz
Exporting app myapp... OK
```

### apps import

Import an application from an export file.

```bash
$ convox cloud apps import [app] -i <machine>
```

**Options:**
- `--file`: Input file path

**Example:**
```bash
$ convox cloud apps import -a myapp -i staging --file myapp-backup.tgz
Importing app myapp... OK
```

### apps info

Get detailed information about an application.

```bash
$ convox cloud apps info [app] -i <machine>
```

**Example:**
```bash
$ convox cloud apps info -a myapp -i production
Name        myapp
Status      running
Generation  3
Locked      false
Release     RABCDEFGHI
```

### apps params

View application parameters.

```bash
$ convox cloud apps params [app] -i <machine>
```

### apps params set

Set application parameters.

```bash
$ convox cloud apps params set <Key=Value> [Key=Value]... -a <app> -i <machine>
```

**Example:**
```bash
$ convox cloud apps params set BuildMemory=2048 -a myapp -i production
Setting parameters... OK
```

## Build Commands

### build

Create a new build for an application.

```bash
$ convox cloud build [dir] -a <app> -i <machine>
```

**Options:**
- `--build-args`: Build-time arguments
- `--description`: Build description
- `--development`: Development build
- `--external`: Use external builder
- `--manifest`: Manifest file (default: convox.yml)
- `--no-cache`: Disable build cache

**Example:**
```bash
$ convox cloud build . -a myapp -i production --description "Feature update"
Packaging source... OK
Uploading source... OK
Starting build... OK
Build:   BABCDEFGHI
Release: RABCDEFGHI
```

### builds

List builds for an application.

```bash
$ convox cloud builds -a <app> -i <machine>
```

**Options:**
- `--limit`: Number of builds to show
- `--watch`: Watch for updates

**Example:**
```bash
$ convox cloud builds -a myapp -i production --limit 5
ID           STATUS    RELEASE      STARTED       ELAPSED  DESCRIPTION
BABCDEFGHI   complete  RABCDEFGHI   1 hour ago    2m       Feature update
BBCDEFGHIJ   complete  RBCDEFGHIJ   2 hours ago   3m
```

### builds export

Export a build.

```bash
$ convox cloud builds export <build> -a <app> -i <machine>
```

**Options:**
- `--file`: Output file path

### builds import

Import a build.

```bash
$ convox cloud builds import -a <app> -i <machine>
```

**Options:**
- `--file`: Input file path

### builds info

Get information about a specific build.

```bash
$ convox cloud builds info <build> -a <app> -i <machine>
```

### builds logs

View logs for a build.

```bash
$ convox cloud builds logs <build> -a <app> -i <machine>
```

**Example:**
```bash
$ convox cloud builds logs BABCDEFGHI -a myapp -i production
Building: .
Step 1/5 : FROM node:18
...
Successfully built abc123def456
```

## Deployment Commands

### deploy

Build and promote in a single command.

```bash
$ convox cloud deploy [dir] -i <machine>
```

**Options:**
- `--app`: Target application (uses directory name if not specified)
- `--build-args`: Build-time arguments
- `--description`: Deployment description
- `--force`: Force deployment
- `--manifest`: Manifest file
- `--no-cache`: Disable build cache

**Example:**
```bash
$ convox cloud deploy . -i production --description "v2.0 release"
Packaging source... OK
Uploading source... OK
Starting build... OK
Build:   BABCDEFGHI
Release: RABCDEFGHI
Promoting RABCDEFGHI... OK
```

## Environment Commands

### env

List environment variables for an application.

```bash
$ convox cloud env -a <app> -i <machine>
```

**Options:**
- `--release`: Specific release
- `--watch`: Watch for updates

**Example:**
```bash
$ convox cloud env -a myapp -i production
DATABASE_URL=postgres://localhost/myapp
REDIS_URL=redis://localhost:6379
NODE_ENV=production
```

### env edit

Edit environment variables interactively.

```bash
$ convox cloud env edit -a <app> -i <machine>
```

**Options:**
- `--promote`: Auto-promote after editing

### env get

Get a specific environment variable.

```bash
$ convox cloud env get <var> -a <app> -i <machine>
```

**Example:**
```bash
$ convox cloud env get DATABASE_URL -a myapp -i production
postgres://localhost/myapp
```

### env set

Set environment variables.

```bash
$ convox cloud env set <key=value> [key=value]... -a <app> -i <machine>
```

**Options:**
- `--promote`: Auto-promote after setting
- `--replace`: Replace all environment variables

**Example:**
```bash
$ convox cloud env set NODE_ENV=production API_KEY=secret -a myapp -i production
Setting NODE_ENV, API_KEY... OK
Release: RCDEFGHIJK
```

### env unset

Remove environment variables.

```bash
$ convox cloud env unset <key> [key]... -a <app> -i <machine>
```

**Options:**
- `--promote`: Auto-promote after unsetting

**Example:**
```bash
$ convox cloud env unset DEBUG_MODE -a myapp -i production
Unsetting DEBUG_MODE... OK
Release: RDEFGHIJKL
```

## Process Management

### exec

Execute a command in a running process.

```bash
$ convox cloud exec <pid> <command> -a <app> -i <machine>
```

**Example:**
```bash
$ convox cloud exec web-abc123 bash -a myapp -i production
/app #
```

### ps

List running processes.

```bash
$ convox cloud ps -a <app> -i <machine>
```

**Options:**
- `--release`: Specific release
- `--service`: Filter by service
- `--watch`: Watch for updates

**Example:**
```bash
$ convox cloud ps -a myapp -i production
ID            SERVICE  STATUS   RELEASE      STARTED     COMMAND
web-abc123    web      running  RABCDEFGHI   1 hour ago  npm start
worker-def456 worker   running  RABCDEFGHI   1 hour ago  npm run worker
```

### ps info

Get information about a specific process.

```bash
$ convox cloud ps info <pid> -a <app> -i <machine>
```

### ps stop

Stop a running process.

```bash
$ convox cloud ps stop <pid> -a <app> -i <machine>
```

**Example:**
```bash
$ convox cloud ps stop web-abc123 -a myapp -i production
Stopping web-abc123... OK
```

### run

Run a one-off command in a new process.

```bash
$ convox cloud run <service> <command> -a <app> -i <machine>
```

**Options:**
- `--cpu`: CPU allocation (millicores)
- `--memory`: Memory allocation (MB)
- `--detach`: Run in background
- `--entrypoint`: Override entrypoint
- `--release`: Specific release

**Example:**
```bash
$ convox cloud run web "rake db:migrate" -a myapp -i production
Running... OK

$ convox cloud run web bash -a myapp -i production
/app # 
```

## Release Management

### releases

List releases for an application.

```bash
$ convox cloud releases -a <app> -i <machine>
```

**Options:**
- `--limit`: Number of releases to show
- `--watch`: Watch for updates

**Example:**
```bash
$ convox cloud releases -a myapp -i production
ID           STATUS  BUILD        CREATED        DESCRIPTION
RCDEFGHIJK           BABCDEFGHI   1 minute ago   env add:API_KEY
RABCDEFGHI   active  BABCDEFGHI   5 minutes ago  v2.0 release
```

### releases create-from

Create a release from existing builds and environments.

```bash
$ convox cloud releases create-from -a <app> -i <machine>
```

**Options:**
- `--build-from`: Source build
- `--env-from`: Source environment
- `--promote`: Auto-promote
- `--use-active-release-build`: Use active release build
- `--use-active-release-env`: Use active release environment

### releases info

Get information about a release.

```bash
$ convox cloud releases info <release> -a <app> -i <machine>
```

### releases manifest

View the manifest for a release.

```bash
$ convox cloud releases manifest <release> -a <app> -i <machine>
```

### releases promote

Promote a release to active.

```bash
$ convox cloud releases promote <release> -a <app> -i <machine>
```

**Options:**
- `--force`: Force promotion

**Example:**
```bash
$ convox cloud releases promote RCDEFGHIJK -a myapp -i production
Promoting RCDEFGHIJK... OK
```

### releases rollback

Rollback to a previous release.

```bash
$ convox cloud releases rollback <release> -a <app> -i <machine>
```

**Options:**
- `--force`: Force rollback

**Example:**
```bash
$ convox cloud releases rollback RABCDEFGHI -a myapp -i production
Rolling back to RABCDEFGHI... OK
```

## Service Management

### services

List services for an application.

```bash
$ convox cloud services -a <app> -i <machine>
```

**Options:**
- `--watch`: Watch for updates

**Example:**
```bash
$ convox cloud services -a myapp -i production
SERVICE  DOMAIN                                    PORTS
web      web.myapp.cloud.convox.com              443:3000
api      api.myapp.cloud.convox.com              443:8080
```

### services restart

Restart a service.

```bash
$ convox cloud services restart <service> -a <app> -i <machine>
```

**Example:**
```bash
$ convox cloud services restart web -a myapp -i production
Restarting web... OK
```

### restart

Restart an entire application.

```bash
$ convox cloud restart -a <app> -i <machine>
```

**Example:**
```bash
$ convox cloud restart -a myapp -i production
Restarting app... OK
```

### scale

Scale a service.

```bash
$ convox cloud scale <service> -a <app> -i <machine>
```

**Options:**
- `--count`: Number of processes
- `--cpu`: CPU per process (millicores)
- `--memory`: Memory per process (MB)
- `--watch`: Watch scaling progress

**Example:**
```bash
$ convox cloud scale web --count 3 --cpu 500 --memory 1024 -a myapp -i production
Scaling web...
OK
```

## Resource Management (Cloud Databases)

Cloud Databases are defined in your `convox.yml` with `provider: aws`. The CLI provides commands to interact with these managed databases.

### resources

List resources for an application.

```bash
$ convox cloud resources -a <app> -i <machine>
```

**Options:**
- `--watch`: Watch for updates

**Example:**
```bash
$ convox cloud resources -a myapp -i production
NAME      TYPE      URL
database  postgres  postgres://user:pass@host:5432/db
cache     redis     redis://host:6379/0
```

### resources console

Open a console for a resource.

```bash
$ convox cloud resources console <resource> -a <app> -i <machine>
```

**Example:**
```bash
$ convox cloud resources console database -a myapp -i production
psql (17.5)
Type "help" for help.
db=#
```

### resources export

Export data from a resource.

```bash
$ convox cloud resources export <resource> -a <app> -i <machine>
```

**Options:**
- `--file`: Output file path

**Example:**
```bash
$ convox cloud resources export database -a myapp -i production --file backup.sql
Exporting data from database... OK
```

### resources import

Import data to a resource.

```bash
$ convox cloud resources import <resource> -a <app> -i <machine>
```

**Options:**
- `--file`: Input file path

**Example:**
```bash
$ convox cloud resources import database -a myapp -i staging --file backup.sql
Importing data to database... OK
```

### resources info

Get information about a resource.

```bash
$ convox cloud resources info <resource> -a <app> -i <machine>
```

### resources proxy

Proxy a local port to a resource.

```bash
$ convox cloud resources proxy <resource> -a <app> -i <machine>
```

**Options:**
- `--port`: Local port
- `--tls`: Enable TLS

**Example:**
```bash
$ convox cloud resources proxy database -a myapp -i production --port 5433
Proxying localhost:5433 to database.internal:5432
```

### resources url

Get the connection URL for a resource.

```bash
$ convox cloud resources url <resource> -a <app> -i <machine>
```

**Example:**
```bash
$ convox cloud resources url database -a myapp -i production
postgres://user:pass@host:5432/db
```

## Monitoring Commands

### logs

Stream logs from an application.

```bash
$ convox cloud logs -a <app> -i <machine>
```

**Options:**
- `--allow-previous`: Include previous container logs
- `--filter`: Filter log output
- `--no-follow`: Don't stream logs
- `--service`: Specific service
- `--since`: Time window (e.g., "2h", "30m")
- `--tail`: Number of lines to show

**Example:**
```bash
$ convox cloud logs -a myapp -i production --service web --since 1h
2024-01-15T10:30:00Z service/web/abc123 GET / 200
2024-01-15T10:30:15Z service/web/abc123 GET /api/users 200
```

## Configuration Management

### config get

Get a configuration value.

```bash
$ convox cloud config get <name> -a <app> -i <machine>
```

### config set

Set a configuration value.

```bash
$ convox cloud config set <name> -a <app> -i <machine>
```

**Options:**
- `--file`: Configuration file
- `--restart`: Restart after setting
- `--value`: Configuration value

### configs

List configurations.

```bash
$ convox cloud configs -a <app> -i <machine>
```

**Options:**
- `--watch`: Watch for updates

## Utility Commands

### cp

Copy files to/from processes.

```bash
$ convox cloud cp <[pid:]src> <[pid:]dst> -a <app> -i <machine>
```

**Options:**
- `--tar-extra`: Extra tar options

**Examples:**
```bash
# Copy from container to local
$ convox cloud cp web-abc123:/app/config.json . -a myapp -i production

# Copy from local to container
$ convox cloud cp ./data.csv web-abc123:/tmp/ -a myapp -i production
```

### test

Run test suite for an application.

```bash
$ convox cloud test -a <app> -i <machine>
```

**Example:**
```bash
$ convox cloud test -a myapp -i staging
Running tests...
✓ All tests passed
```

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

# Monitor logs
$ convox cloud logs -a myapp -i prod --follow
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
- Documentation: [docs.convox.com](https://docs.convox.com)
- Community: [community.convox.com](https://community.convox.com)
- Email: cloud-support@convox.com