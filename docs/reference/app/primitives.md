# Primitives

Primitives are the basic building blocks of an [App](../app.md).

Primitives are defined in [`convox.yml`](convox.yml.md) and can be easily composed to provide
useful functionality that lets you focus on the bits that make your [App](../app.md) unique.

## Available Primitives

| Primitive                          | Description                                                                                         |
| :--------------------------------- | :-------------------------------------------------------------------------------------------------- |
| [Balancer](primitives/balancer.md) | Custom TCP load balancers in front of a [Service](primitives/service.md)                            |
| [Build](primitives/build.md)       | Compiled version of a codebase                                                                      |
| [Object](primitives/object.md)     | Blob/file storage                                                                                   |
| [Process](primitives/process.md)   | Running containers created by running a command on a [Release](primitives/build.md)                 |
| [Release](primitives/release.md)   | Units of deployment consisting of a [Build](primitives/build.md) and a set of environment variables |
| [Resource](primitives/resource.md) | Network-accessible external services (e.g. Postgres)                                                |
| [Service](primitives/service.md)   | Horizontally-scalable collections of durable [Processes](primitives/process.md)                     |
| [Timer](primitives/timer.md)       | Runs one-off [Processes](primitives/process.md) on a scheduled interval                             |

## Planned Primitives

| Primitive | Description                                 |
| :-------- | :------------------------------------------ |
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
