# Primitives

Primitives are the building blocks available to build an [App](app) on Convox. Apps are deployed and run on your [Rack](rack), the platform overlaid on your cloud infrastructure.

Primitives can be easily composed to provide useful functionality that lets you
focus on the things that make your [App](app) unique.

## App Primitives

| Primitive                   | Description                                                                                  |
|:----------------------------|:---------------------------------------------------------------------------------------------|
| [Balancer](app/balancer.md) | Custom TCP load balancers in front of a [Service](app/service.md)                            |
| [Build](app/build.md)       | Compiled version of a codebase                                                               |
| [Object](app/object.md)     | Blob/file storage                                                                            |
| [Process](app/process.md)   | Running containers created by running a command on a [Release](app/build.md)                 |
| [Release](app/release.md)   | Units of deployment consisting of a [Build](app/build.md) and a set of environment variables |
| [Resource](app/resource.md) | Network-accessible external services (e.g. Postgres)                                         |
| [Service](app/service.md)   | Horizontally-scalable collections of durable [Processes](app/process.md)                     |
| [Timer](app/timer.md)       | Runs one-off [Processes](app/process.md) on a scheduled interval                             |

### Coming Soon

| Primitive | Description                                 |
|:----------|:--------------------------------------------|
| Cache     | Store data with timed expiration            |
| Feature   | Toggleable feature flags                    |
| Identity  | User, group, and permission management      |
| Key       | Encrypt and decrypt data                    |
| Lock      | Coordinate exclusive access                 |
| Mail      | Send and receive email                      |
| Metric    | Store and analyze time-series data          |
| Queue     | An expandable list of items to be processed |
| Search    | Full-text indexing of data                  |
| Stream    | Subscribable one-to-many data stream        |
| Table     | Indexable rows of key/value data            |

## Rack Primitives

[Racks](rack) contain a couple of simple primitives, that enable the Convox processes and your own services to run seamlessly.

| Primitive                    | Description                                                         |
|:-----------------------------|:--------------------------------------------------------------------|
| [Instance](rack/instance.md) | Node that provides capacity for running [Processes](app/process.md) |
| [Registry](rack/registry.md) | External image repository                                           |