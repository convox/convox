# Convox Installer

[Convox Rack](https://github.com/convox/convox) is an open-source PaaS based on Kubernetes.

For more information you can check out the following resources:

- [Homepage](https://convox.com)
- [Getting Started Guide](https://docs.convox.com/introduction/getting-started)
- [FAQ](https://docs.convox.com/introduction/faq)
- [Community Forums](https://community.convox.com/)

## Installation

Convox uses [Terraform](https://www.terraform.io/) for installation.

Go into the relevant subdirectory of this repository and follow the instructions in the README.

| Cloud Provider      | Subdirectory     |
|:--------------------|:-----------------|
| Amazon Web Services | [aws](aws)      |
| Azure               | [azure](azure)  |
| Digital Ocean       | [do](do)        |
| Google Cloud        | [gcp](gcp)      |

## Features

* [Build and Release Management](https://docs.convox.com/deployment/builds)
* [Secrets Management](https://docs.convox.com/application/environment)
* [Resource Management](https://docs.convox.com/use-cases/resources) \(Postgres/Redis/etc..\)
* [Automated Rollbacks](https://docs.convox.com/deployment/rolling-back)
* [Autoscaling](https://docs.convox.com/deployment/scaling)
* [Timers/Cron Jobs](https://docs.convox.com/application/timers)
* [One-off Commands](https://docs.convox.com/management/one-off-commands)

## License

Apache 2.0
