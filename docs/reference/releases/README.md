---
title: "Releases"
slug: releases
url: /reference/releases
---
# Releases

Convox follows semantic versioning (`major.minor.patch`). Minor versions introduce new features, Kubernetes upgrades, and dependency updates. Patch versions contain bug fixes, small improvements, and backported features.

New releases are published on the [Convox GitHub Releases](https://github.com/convox/convox/releases) page. This section provides a structured, browsable history of all releases from 3.15.0 onward.

## Kubernetes Version Compatibility

Each Convox minor version targets a specific Kubernetes version. Upgrading your rack to a new minor version will upgrade the underlying Kubernetes cluster.

| Convox Version | Kubernetes Version | Initial Release |
|----------------|--------------------|-----------------|
| 3.23.x         | 1.33               | 2025-11-05      |
| 3.22.x         | 1.32               | 2025-08-14      |
| 3.21.x         | 1.31               | 2025-04-02      |
| 3.20.x         | 1.30               | 2025-02-25      |
| 3.19.x         | 1.29               | 2024-09-20      |
| 3.18.x         | 1.28               | 2024-04-08      |
| 3.17.x         | 1.28               | 2024-03-20      |
| 3.16.x         | 1.27               | 2024-02-14      |
| 3.15.x         | 1.26               | 2024-01-15      |

## Release History

- [3.23 Releases](/reference/releases/3-23) - VPA, KEDA, K8s 1.33, Azure node groups
- [3.22 Releases](/reference/releases/3-22) - Build args, K8s 1.32, release cleanup
- [3.21 Releases](/reference/releases/3-21) - K8s 1.31, GPU support, node groups
- [3.20 Releases](/reference/releases/3-20) - K8s 1.30, component upgrades
- [3.19 Releases](/reference/releases/3-19) - K8s 1.29, initContainers, startup probes
- [3.18 Releases](/reference/releases/3-18) - RDS/ElastiCache, EFS volumes, NLB
- [3.17 Releases](/reference/releases/3-17) - K8s 1.28
- [3.16 Releases](/reference/releases/3-16) - K8s 1.27, emptyDir volumes
- [3.15 Releases](/reference/releases/3-15) - BuildKit v0.12.4, VPC CNI v1.14

## Updating Your Rack

To update your rack to the latest version:

```bash
convox rack update
```

To update to a specific version:

```bash
convox rack update 3.23.4
```

For detailed update instructions, see [Updating a Rack](/management/cli-rack-management#updating-to-the-latest-version).

## See Also

- [GitHub Releases](https://github.com/convox/convox/releases) for the latest patch notes
- [Changes](/help/changes) for v2 to v3 migration reference
- [CLI update](/reference/cli/update) command reference
