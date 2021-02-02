# One-off Commands

Convox allows you to execute one-off commands on your [App](../reference/primitives/app). This can be used
for starting a shell for debugging purposes or running administrative commands such as database migrations.

## Spawning a new Process

Using `convox run` will start a new [Process](../reference/primitives/app/process.md) of the specified
[Service](../reference/primitives/app/service.md) on your current Rack and run your command inside the new Process.

### Running Interactively

Running an interactive process will start a [Process](../reference/primitives/app/process.md) and connect
your local terminal so that you can run commands and see output:

    $ convox run web bash
    root@web#

### Running Detached

Running detached is useful for long-running tasks that you don't want to be disrupted:

    $ convox run web bin/cleanup-database --detach
    Running detached process... OK, web-s43xf

> The output of detached [Processes](../reference/primitives/app/process.md) will appear in the
> [application logs](../configuration/logging.md)

## Running a command in an existing Process

Using `convox exec` will run a command inside an existing [Process](../reference/primitives/app/process.md).
This can be useful for debugging a running [Process](../reference/primitives/app/process.md).

    $ convox ps
    ID                    SERVICE  STATUS   RELEASE     STARTED         COMMAND
    web-6844dc6f45-9wdss  web      running  RABCDEFGHI  14 minutes ago  bin/web
    web-6844dc6f45-mj9mp  web      running  RABCDEFGHI  14 minutes ago  bin/web

    $ convox exec web-6844dc6f45-9wdss bash
    root@web#

You can also use a [Service](../reference/primitives/app/service.md) name with `convox exec` to select
a random [Process](../reference/primitives/app/process.md) of that Service.

    $ convox exec web bash
    root@web#
