# Convox

Convox is an open-source [PaaS](https://en.wikipedia.org/wiki/Platform_as_a_service) based
on Kubernetes available for multiple cloud providers.

## Supported Clouds

- Amazon Web Services
- Digital Ocean
- Google Cloud
- Microsoft Azure

## Getting Started

- [Introduction](docs/getting-started/introduction.md)

## Installation

* [Command Line Interface](docs/installation/cli.md)
* [Development Rack](docs/installation/development-rack)
* [Production Rack](docs/installation/production-rack)

## Features

* [Release Management](docs/reference/primitives/app/release.md)
* [Secrets Management](docs/configuration/environment.md)
* [Load Balancing](docs/configuration/load-balancers.md) (automatic SSL)
* [Service Discovery](docs/configuration/service-discovery.md)
* [Resource Management](docs/reference/primitives/app/resource/README.md) (Postgres, Redis, etc)
* [Automated Rollbacks](docs/deployment/rollbacks.md)
* [Autoscaling](docs/deployment/scaling.md)
* [Scheduled Runs](docs/reference/primitives/app/timer.md) (cron)
* [One-off Commands](docs/management/run.md)

## Resources

- [Homepage](https://convox.com)
- [Stack Overflow](https://stackoverflow.com/questions/tagged/convox)

## Development Tips

When testing new changes, a good way of adding them to a test rack is to build the image locally,push to a public repo and update the k8s deployment api:

```sh
docker build -t user/convox:tag .
docker push user/convox:tag
kubectl set image deploy api system=user/convox:tag -n rackName-system
```

If testing new changes in terraform, install the rack using the following command to have the `/terraform` folder mapped to the rack tf manifest.

```sh
/convox: CONVOX_TERRAFORM_SOURCE=$PWD//terraform/system/%s convox rack install aws rack1
```

After saving your changes, go to `~/.config/convox/racks/rack1` and run `terraform apply`

## License

- [Apache 2.0](LICENSE)
