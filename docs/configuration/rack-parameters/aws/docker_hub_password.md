---
title: "docker_hub_password"
slug: docker_hub_password
url: /configuration/rack-parameters/aws/docker_hub_password
---

# docker_hub_password

## Description
The `docker_hub_password` parameter sets the Docker Hub authentication token for the rack. Must be used alongside `docker_hub_username` to enable authenticated Docker Hub pulls.

## Default Value
Not set (anonymous Docker Hub access).

## Use Cases
- **Avoid Rate Limits**: Authenticated Docker Hub pulls have significantly higher rate limits than anonymous pulls.
- **Private Images**: Access private Docker Hub repositories during builds and deployments.

## Setting Parameters
Set both `docker_hub_username` and `docker_hub_password` together:
```bash
$ convox rack params set docker_hub_username=myuser docker_hub_password=dckr_pat_xxxxx -r rackName
Setting parameters... OK
```

Use a read-only access token generated from [Docker Hub Account Settings](https://hub.docker.com/settings/security) rather than your account password.

## Additional Information
Requires rack version 3.18.8 or later.

See [docker_hub_username](/configuration/rack-parameters/aws/docker_hub_username) and [Using Docker Credentials in Builds](/reference/primitives/app/build#using-docker-credentials-in-builds) for more details.
