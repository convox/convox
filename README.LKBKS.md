# LKBKS Convox Fork

This is a fork of [Convox](https://github.com/convox/convox) intended to be used temporarily to unblock new app releases until official support for k8s 1.33 drops.

Based on Convox 3.22.4 with K8s 1.33 compatibility

## Build & Deploy

- Note: updated racks to 3.22.4 first.

### development
```bash
# Build custom image
docker build --platform linux/amd64 -t us-east1-docker.pkg.dev/lookbooks-legacy/legacy/convox:3.22.4-k8s-1.33 .

# Push to Artifact Registry
docker push us-east1-docker.pkg.dev/lookbooks-legacy/legacy/convox:3.22.4-k8s-1.33

# Deploy to cluster
kubectl set image -n legacy-development-system deployment/atom system=us-east1-docker.pkg.dev/lookbooks-legacy/legacy/convox:3.22.4-k8s-1.33 --context=lookbooks-legacy-development
kubectl set image -n legacy-development-system deployment/api system=us-east1-docker.pkg.dev/lookbooks-legacy/legacy/convox:3.22.4-k8s-1.33 --context=lookbooks-legacy-development
```

### production
```bash
# Build custom image
docker build --platform linux/amd64 -t us-east1-docker.pkg.dev/lookbooks-legacy-production/legacy/convox:3.22.4-k8s-1.33 .

# Push to Artifact Registry
docker push us-east1-docker.pkg.dev/lookbooks-legacy-production/legacy/convox:3.22.4-k8s-1.33

# Deploy to cluster
kubectl set image -n legacy-production-system deployment/atom system=us-east1-docker.pkg.dev/lookbooks-legacy-production/legacy/convox:3.22.4-k8s-1.33 --context=lookbooks-legacy-production
kubectl set image -n legacy-production-system deployment/api system=us-east1-docker.pkg.dev/lookbooks-legacy-production/legacy/convox:3.22.4-k8s-1.33 --context=lookbooks-legacy-production
```
