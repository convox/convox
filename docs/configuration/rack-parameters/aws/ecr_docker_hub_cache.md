---
title: "ecr_docker_hub_cache"
slug: ecr_docker_hub_cache
url: /configuration/rack-parameters/aws/ecr_docker_hub_cache
---

# ecr_docker_hub_cache

## Description
The `ecr_docker_hub_cache` parameter enables an ECR pull-through cache for Docker Hub images. When enabled, Convox-managed resource pods (Redis, Postgres, MySQL, MariaDB, Memcached, PostGIS) pull Docker Hub images through your account's ECR registry, which transparently caches them. This eliminates Docker Hub rate limit (429 Too Many Requests) errors on clusters with node churn, where fresh nodes frequently need to pull images.

Docker Hub credentials are required. ECR pull-through must authenticate to Docker Hub when populating the cache; anonymous pulls are rejected by ECR when establishing the pull-through rule.

## Default Value
The default value for `ecr_docker_hub_cache` is `false`.

## Use Cases
- **Eliminate Docker Hub Rate Limits**: Avoid 429 errors caused by anonymous or authenticated pull rate limits, especially on clusters where nodes are frequently replaced (e.g., Spot instances, Karpenter-managed nodes).
- **Faster Image Pulls**: After the first pull, images are served from ECR in the same region, significantly reducing pull times.
- **Reduced External Dependencies**: Once cached, images are available even if Docker Hub experiences outages.

## Setting Parameters

Enable the cache together with Docker Hub credentials in a single `params set` call:

```bash
$ convox rack params set ecr_docker_hub_cache=true docker_hub_username=myuser docker_hub_password=dckr_pat_xxxxx -r rackName
Setting parameters... OK
```

If `docker_hub_username` and `docker_hub_password` are already set on the rack, you can enable the cache on its own:

```bash
$ convox rack params set ecr_docker_hub_cache=true -r rackName
Setting parameters... OK
```

Enabling the cache without both credentials present (either already applied or being set in the same call) returns a validation error from the CLI:

```text
ecr_docker_hub_cache=true requires docker_hub_username and docker_hub_password.
  Set all three: convox rack params set ecr_docker_hub_cache=true docker_hub_username=USER docker_hub_password=TOKEN
  Or set docker_hub_username and docker_hub_password first
```

We recommend using a [Docker Hub personal access token](https://docs.docker.com/security/for-developers/access-tokens/) with read-only scope rather than an account password.

## Disabling the Cache

```bash
$ convox rack params set ecr_docker_hub_cache=false -r rackName
Setting parameters... OK
```

Disabling tears down the ECR pull-through cache rule, the Secrets Manager secret holding the Docker Hub credentials, and the IAM policy granting nodes `ecr:CreateRepository` and `ecr:BatchImportUpstreamImage` permissions. Resource pods revert to pulling directly from Docker Hub on their next deploy.

## Additional Information

- When enabled, Convox creates:
  - An ECR pull-through cache rule with prefix `docker-hub-<rack-name>` pointing to `registry-1.docker.io`
  - An AWS Secrets Manager secret storing the Docker Hub credentials
  - An IAM policy scoped to the cache prefix granting cluster nodes the permissions needed to lazily create cache repositories and import upstream images on first pull
- Resource images are automatically rewritten to use the ECR cache URL. For example, `redis:4.0.10` becomes `<account_id>.dkr.ecr.<region>.amazonaws.com/docker-hub-<rack-name>/library/redis:4.0.10`.
- Docker Hub "library" images (redis, postgres, mysql, mariadb, memcached) get a `library/` prefix in the ECR path per Docker Hub's canonical convention. Namespaced images (e.g., `postgis/postgis`) keep their full path.
- Custom image overrides on resource definitions are always respected — user-supplied images are never rewritten, even when the cache is enabled.
- Each rack has its own ECR cache prefix (`docker-hub-<rack-name>`) and its own IAM scope, so multiple racks in the same AWS account cache independently.
- After enabling the cache, existing resource pods continue to use their previous image references until the next deploy. Redeploy apps that depend on Convox-managed resources to move them onto the cache.
- Standard ECR storage costs apply for cached images.

## See Also

- [docker_hub_username](/configuration/rack-parameters/aws/docker_hub_username) for Docker Hub authentication
- [docker_hub_password](/configuration/rack-parameters/aws/docker_hub_password) for the access token
