---
title: "ecr_docker_hub_cache"
slug: ecr_docker_hub_cache
url: /configuration/rack-parameters/aws/ecr_docker_hub_cache
---

# ecr_docker_hub_cache

## Description
The `ecr_docker_hub_cache` parameter enables an ECR pull-through cache for Docker Hub images. When enabled, resource pods (Redis, Postgres, MySQL, MariaDB, Memcached, PostGIS) pull Docker Hub images through your account's ECR registry, which transparently caches them. This eliminates Docker Hub rate limit (429 Too Many Requests) errors on clusters with node churn, where fresh nodes frequently need to pull images.

## Default Value
The default value for `ecr_docker_hub_cache` is `false`.

## Use Cases
- **Eliminate Docker Hub Rate Limits**: Avoid 429 errors caused by anonymous or authenticated pull rate limits, especially on clusters where nodes are frequently replaced (e.g., Spot instances, Karpenter-managed nodes).
- **Faster Image Pulls**: After the first pull, images are served from ECR in the same region, significantly reducing pull times.
- **Reduced External Dependencies**: Once cached, images are available even if Docker Hub experiences outages.

## Setting Parameters
To enable the ECR pull-through cache:
```bash
$ convox rack params set ecr_docker_hub_cache=true -r rackName
Setting parameters... OK
```

For best results, also set Docker Hub credentials so upstream cache misses use authenticated pulls with higher rate limits:
```bash
$ convox rack params set ecr_docker_hub_cache=true docker_hub_username=myuser docker_hub_password=dckr_pat_xxxxx -r rackName
Setting parameters... OK
```

The cache also works without Docker Hub credentials (anonymous upstream pulls), but cache miss pulls are subject to Docker Hub's anonymous rate limits.

## Additional Information
- When enabled, Convox creates:
  - An ECR pull-through cache rule with prefix `docker-hub-<rack-name>` pointing to `registry-1.docker.io`
  - An AWS Secrets Manager secret with Docker Hub credentials (if `docker_hub_username` and `docker_hub_password` are set)
  - An IAM policy granting cluster nodes permission to create ECR repositories and import upstream images
- Resource images are automatically rewritten to use the ECR cache URL. For example, `redis:4.0.10` becomes `<account_id>.dkr.ecr.<region>.amazonaws.com/docker-hub-<rack-name>/library/redis:4.0.10`.
- Docker Hub "library" images (redis, postgres, mysql, mariadb, memcached) get a `library/` prefix in the ECR path per Docker Hub convention. Non-library images (e.g., `postgis/postgis`) use their full path.
- ECR repositories are created automatically on first pull — no manual setup is needed.
- Cached images follow your ECR lifecycle policies and region configuration.
- This feature complements `docker_hub_username` and `docker_hub_password` — those parameters authenticate direct pulls for build pods and service images, while `ecr_docker_hub_cache` specifically caches the images used by Convox-managed resource pods.
- Standard ECR storage costs apply for cached images.
- Each rack gets its own ECR pull-through cache rule with a rack-specific prefix (`docker-hub-<rack-name>`), so multiple racks in the same AWS account can each enable this feature independently.
