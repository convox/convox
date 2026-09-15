---
title: "builds"
description: "The convox builds command lists builds and manages build operations such as info, logs, cancelling a running build, and importing or exporting builds."
slug: builds
url: /reference/cli/builds
---
# builds

## builds

List builds

### Usage
```bash
    convox builds
```
### Examples
```bash
    $ convox builds
    ID           STATUS    RELEASE      STARTED       ELAPSED  DESCRIPTION
    BABCDEFGHIJ  complete  RABCDEFGHIJ  1 week ago    17s
    BBCDEFGHIJK  complete  RBCDEFGHIJK  1 week ago    9s       My latest build
    BCDEFGHIJKL  failed                 1 week ago    3s       My latest build
```
## builds cancel

Cancel a running build

> `convox builds cancel` requires rack version 3.25.7 or later. An earlier rack returns `convox builds cancel requires rack version 3.25.7 or later`. The subcommand ships in the `convox` CLI at `3.25.7`; run `sudo convox update` to get it.

### Usage
```bash
    convox builds cancel <build>
```
### Examples
```bash
    $ convox builds cancel BABCDEFGHIJ
    Cancelling build BABCDEFGHIJ... OK
```

Cancelling deletes the Build's pod and sets the Build to `failed`. There is no separate cancelled status. `convox builds info` carries the reason:

```bash
    $ convox builds info BABCDEFGHIJ
    Id           BABCDEFGHIJ
    Status       failed
    Reason       cancelled by user@example.com
    Release
    Started      3 minutes ago
    Elapsed      3m12s
```

The actor is the Console user's email on a Rack reached through Console, and `rack-password` on a direct connection authenticated with the Rack password.

A Build whose pod cannot be scheduled stays `running` until it is cancelled; nothing on the Rack moves it out of that state.

Only a Build the Rack is running in a build pod can be cancelled. A Build that has finished, one already cancelled, and one whose image comes from outside the Rack (`convox build --external`, `convox builds import-image`) are all refused:

```bash
    $ convox builds cancel BABCDEFGHIJ
    Cancelling build BABCDEFGHIJ... ERROR: build BABCDEFGHIJ is not running
```

Cancelling does not change the configuration that left the Build unschedulable. When [`BuildLabels`](/configuration/app-parameters/aws/BuildLabels) or [`BuildArch`](/configuration/app-parameters/aws/BuildArch) selects nodes the Rack does not have, the next Build stays pending the same way until the parameter changes.

[`convox apps cancel`](/reference/cli/apps#apps-cancel) is a different command: it cancels an App deploy in progress and does not affect Builds.

## builds export

Export a build

### Usage
```bash
    convox builds export <build>
```
### Flags

| Flag | Short | Description |
| ---- | ----- | ----------- |
| `--file` | `-f` | Export to file |

### Examples
```bash
    $ convox builds export BABCDEFGHIJ --file build.tgz
    Exporting build... OK
```
## builds import

Import a build

### Usage
```bash
    convox builds import
```
### Flags

| Flag | Short | Description |
| ---- | ----- | ----------- |
| `--file` | `-f` | Import from file |

### Examples
```bash
    $ convox builds import --file output.tgz
    Importing build... OK, RFGHIJKLMNOP
```
## builds info

Get information about a build

### Usage
```bash
    convox builds info <build>
```
### Examples
```bash
    $ convox builds info BABCDEFGHIJ
    Id           BABCDEFGHIJ
    Status       complete
    Release      RABCDEFGHIJ
    Description  My latest build
    Started      1 week ago
    Elapsed      17s
```
## builds logs

Get logs for a build

### Usage
```bash
    convox builds logs <build>
```
### Examples
```bash
    $ convox builds logs BABCDEFGHIJ
    Authenticating https://index.docker.io/v1/: Login Succeeded
    Authenticating 1234567890.dkr.ecr.us-east-1.amazonaws.com: Login Succeeded
    Building: .
    ...
    ...
    Running: docker tag convox/myapp:web.BABCDEFGHI 1234567890.dkr.ecr.us-east-1.amazonaws.com/test-regis-1mjiluel3aiv3:web.BABCDEFGHI
    Running: docker push 1234567890.dkr.ecr.us-east-1.amazonaws.com/test-regis-1mjiluel3aiv3:web.BABCDEFGHI
```

A Build that did not finish has no stored logs, because the build pod uploads them when it finishes. `convox builds logs` on a cancelled or interrupted Build returns `unable to read logs for build: <build>`.

## See Also

- [Build](/reference/primitives/app/build) for build concepts and build arguments
- [Deploy](/reference/cli/deploy) for building and promoting in one step