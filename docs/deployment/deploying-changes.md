# Deploying Changes

To deploy changes to your code you will first create a [Release](../reference/primitives/app/release.md)
and then promote it.

Promoting a Release will begin a [rolling deployment](rolling-updates.md) that will continue
until the new Release is active on all [Processes](../reference/primitives/app/process.md) or
has been completely rolled back.

## One Step

To create a [Release](../reference/primitives/app/release.md) and promote it in one step, use `convox deploy`:

    $ convox deploy -a myapp
    Packaging source... OK
    Uploading source... OK
    Starting build... OK
    ...
    Build: BABCDEFGHI
    Release: RBCDEFGHIJ
    Promoting RBCDEFGHIJ...
    2019-01-01T00:00:49Z system/k8s/atom/app Status: Running => Pending
    2019-01-01T00:00:51Z system/k8s/web Scaled up replica set web-745f845dc to 1
    2019-01-01T00:00:51Z system/k8s/web-745f845dc Created pod: web-745f845dc-rzl2q
    2019-01-01T00:00:51Z system/k8s/web-745f845dc-rzl2q Successfully assigned convox-myapp/web-745f845dc-rzl2q to instance-0a1b2c3d4e5f
    2019-01-01T00:00:51Z system/k8s/web-745f845dc-rzl2q Pulling image "registry.host/convox/myapp:web.BABCDEFGHI"
    2019-01-01T00:00:53Z system/k8s/atom/app Status: Pending => Updating
    2019-01-01T00:00:55Z system/k8s/web-745f845dc-rzl2q Successfully pulled image "registry.host/convox/myapp:web.BABCDEFGHI"
    2019-01-01T00:00:56Z system/k8s/web-745f845dc-rzl2q Created container main
    2019-01-01T00:00:56Z system/k8s/web-745f845dc-rzl2q Started container main
    OK

## Two Steps

You can also perform the steps of creating the [Release](../reference/primitives/app/release.md) and
promoting it with two different commands. This is useful if you would like to make changes to
[Environment Variables](../configuration/environment.md) or [run migrations](../management/run.md)
against the new [Release](../reference/primitives/app/release.md) before it is pushed live.

    $ convox build -a myapp
    Packaging source... OK
    Uploading source... OK
    Starting build... OK
    ...
    Build: BABCDEFGHI
    Release: RBCDEFGHIJ

    $ convox releases promote RCDEFGHIJK -a myapp
    Promoting RCDEFGHIJK...
    2019-01-01T00:00:49Z system/k8s/atom/app Status: Running => Pending
    2019-01-01T00:00:51Z system/k8s/web Scaled up replica set web-745f845dc to 1
    2019-01-01T00:00:51Z system/k8s/web-745f845dc Created pod: web-745f845dc-rzl2q
    2019-01-01T00:00:51Z system/k8s/web-745f845dc-rzl2q Successfully assigned convox-myapp/web-745f845dc-rzl2q to instance-0a1b2c3d4e5f
    2019-01-01T00:00:51Z system/k8s/web-745f845dc-rzl2q Pulling image "registry.host/convox/myapp:web.BABCDEFGHI"
    2019-01-01T00:00:53Z system/k8s/atom/app Status: Pending => Updating
    2019-01-01T00:00:55Z system/k8s/web-745f845dc-rzl2q Successfully pulled image "registry.host/convox/myapp:web.BABCDEFGHI"
    2019-01-01T00:00:56Z system/k8s/web-745f845dc-rzl2q Created container main
    2019-01-01T00:00:56Z system/k8s/web-745f845dc-rzl2q Started container main
    OK