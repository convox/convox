---
title: "docker_hub_username"
slug: docker_hub_username
url: /configuration/rack-parameters/aws/docker_hub_username
---

# docker_hub_username

## Description
The `docker_hub_username` parameter configures Docker Hub authentication for the rack. When set alongside `docker_hub_password`, all image pulls from Docker Hub are authenticated, avoiding Docker Hub's anonymous pull rate limits.

## Default Value
Not set (anonymous Docker Hub access).

## Use Cases
- **Avoid Rate Limits**: Docker Hub enforces rate limits on anonymous image pulls (100 pulls per 6 hours per IP). Authenticated pulls have significantly higher limits.
- **Private Images**: Pull images from private Docker Hub repositories during builds and deployments.

## Setting Parameters
Set both `docker_hub_username` and `docker_hub_password` together:
```bash
$ convox rack params set docker_hub_username=myuser docker_hub_password=dckr_pat_xxxxx -r rackName
Setting parameters... OK
```

Generate a read-only access token from [Docker Hub Account Settings](https://hub.docker.com/settings/security) rather than using your account password.

## Additional Information
When both credentials are set, the rack creates a Kubernetes image pull secret that authenticates all Docker Hub pulls across the cluster. This applies to build pods, `convox run` pods, service deployments, resource deployments (Redis, Postgres, MySQL, MariaDB, Memcached, PostGIS), and timer CronJobs.

See [Using Docker Credentials in Builds](/reference/primitives/app/build#using-docker-credentials-in-builds) for more details.

## See Also

- [docker_hub_password](/configuration/rack-parameters/aws/docker_hub_password) for the access token parameter
- [ecr_docker_hub_cache](/configuration/rack-parameters/aws/ecr_docker_hub_cache) for an ECR pull-through cache that complements authenticated pulls
